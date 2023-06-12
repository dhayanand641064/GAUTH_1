package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("No .env file found")
	}
}

func main() {
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/login/github/", githubLoginHandler)
	http.HandleFunc("/login/github/callback", githubCallbackHandler)
	http.HandleFunc("/loggedin", func(w http.ResponseWriter, r *http.Request) {
		githubData := r.URL.Query().Get("githubData")
		loggedinHandler(w, r, githubData)
	})

	fmt.Println("[ UP ON PORT 3000 ]")
	log.Panic(http.ListenAndServe(":3000", nil))
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, `<a href="/login/github/">LOGIN</a>`)
}

func loggedinHandler(w http.ResponseWriter, r *http.Request, githubData string) {
	if githubData == "" {
		// Unauthorized response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"error": "Unauthorized"}`)
		return
	}

	// Process authorized response
	w.Header().Set("Content-Type", "application/json")

	var prettyJSON bytes.Buffer
	parserr := json.Indent(&prettyJSON, []byte(githubData), "", "\t")
	if parserr != nil {
		// JSON parse error
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error": "JSON parse error"}`)
		return
	}

	fmt.Fprintf(w, string(prettyJSON.Bytes()))
}

func githubLoginHandler(w http.ResponseWriter, r *http.Request) {
	githubClientID := getGithubClientID()
	redirectURL := fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=user,read:org", githubClientID, "http://localhost:3000/login/github/callback")
	http.Redirect(w, r, redirectURL, 301)
}

func githubCallbackHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	githubAccessToken := getGithubAccessToken(code)
	githubData := getGithubData(githubAccessToken)
	githubOrgs := getGithubOrganizations(githubAccessToken)

	response := struct {
		GithubData string   `json:"githubData"`
		GithubOrgs []string `json:"githubOrgs"`
	}{
		GithubData: githubData,
		GithubOrgs: githubOrgs,
	}

	responseJSON, _ := json.Marshal(response)

	http.Redirect(w, r, "/loggedin?githubData="+string(responseJSON), http.StatusSeeOther)
}

func getGithubData(accessToken string) string {
	req, reqerr := http.NewRequest("GET", "https://api.github.com/user", nil)
	if reqerr != nil {
		log.Panic("API Request creation failed")
	}

	authorizationHeaderValue := fmt.Sprintf("token %s", accessToken)
	req.Header.Set("Authorization", authorizationHeaderValue)

	resp, resperr := http.DefaultClient.Do(req)
	if resperr != nil {
		log.Panic("Request failed")
	}

	respbody, _ := ioutil.ReadAll(resp.Body)

	return string(respbody)
}

func getGithubAccessToken(code string) string {
	clientID := getGithubClientID()
	clientSecret := getGithubClientSecret()

	requestBodyMap := map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"code":          code,
	}

	requestJSON, _ := json.Marshal(requestBodyMap)

	req, reqErr := http.NewRequest("POST", "https://github.com/login/oauth/access_token", bytes.NewBuffer(requestJSON))
	if reqErr != nil {
		log.Panic("Request creation failed:", reqErr)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, respErr := http.DefaultClient.Do(req)
	if respErr != nil {
		log.Panic("Request failed:", respErr)
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)

	type githubAccessTokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
	}

	var ghResp githubAccessTokenResponse
	json.Unmarshal(respBody, &ghResp)

	return ghResp.AccessToken
}

func getGithubClientID() string {
	githubClientID, exists := os.LookupEnv("CLIENT_ID")
	if !exists {
		log.Fatal("Github Client ID not defined in .env file")
	}
	return githubClientID
}

func getGithubClientSecret() string {
	githubClientSecret, exists := os.LookupEnv("CLIENT_SECRET")
	if !exists {
		log.Fatal("Github Client Secret not defined in .env file")
	}
	return githubClientSecret
}

func getGithubOrganizations(accessToken string) []string {
	req, reqerr := http.NewRequest("GET", "https://api.github.com/user/orgs", nil)
	if reqerr != nil {
		log.Panic("API Request creation failed")
	}

	authorizationHeaderValue := fmt.Sprintf("token %s", accessToken)
	req.Header.Set("Authorization", authorizationHeaderValue)

	resp, resperr := http.DefaultClient.Do(req)
	if resperr != nil {
		log.Panic("Request failed")
	}

	defer resp.Body.Close()
	respbody, _ := ioutil.ReadAll(resp.Body)

	type githubOrg struct {
		Login string `json:"login"`
	}

	var orgs []githubOrg
	json.Unmarshal(respbody, &orgs)

	orgNames := make([]string, len(orgs))
	for i, org := range orgs {
		orgNames[i] = org.Login
	}

	return orgNames
}
