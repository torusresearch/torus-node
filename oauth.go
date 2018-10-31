package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type AuthBodyGoogle struct {
	Azp string `json:"azp"`
	Sub string `json:"sub"`
}

func main() {
	// ctx := context.Background()
	// conf := &oauth2.Config{
	// 	ClientID:     "876733105116-i0hj3s53qiio5k95prpfmj0hp0gmgtor.apps.googleusercontent.com",
	// 	ClientSecret: "n8_vhkr52qtQSKuDnAUdUlc2",
	// 	Scopes:       []string{googleOauth.UserinfoProfileScope, googleOauth.UserinfoEmailScope},
	// 	Endpoint: oauth2.Endpoint{
	// 		AuthURL:  "https://provider.com/o/oauth2/auth",
	// 		TokenURL: "https://provider.com/o/oauth2/token",
	// 	},
	// }
	// // Redirect user to consent page to ask for permission
	// // for the scopes specified above.
	// url := conf.AuthCodeURL("state", oauth2.AccessTypeOffline)
	// fmt.Printf("Visit the URL for the auth dialog: %v", url)

	// // Use the authorization code that is pushed to the redirect
	// // URL. Exchange will do the handshake to retrieve the
	// // initial access token. The HTTP Client returned by
	// // conf.Client will refresh the token as necessary.
	// var code string
	// if _, err := fmt.Scan(&code); err != nil {
	// 	log.Fatal(err)
	// }
	// tok, err := conf.Exchange(ctx, code)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	idToken := "eyJhbGciOiJSUzI1NiIsImtpZCI6IjgyODlkNTQyODBiNzY3MTJkZTQxY2QyZWY5NTk3MmIxMjNiZTlhYzAiLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJhY2NvdW50cy5nb29nbGUuY29tIiwiYXpwIjoiODc2NzMzMTA1MTE2LWkwaGozczUzcWlpbzVrOTVwcnBmbWowaHAwZ21ndG9yLmFwcHMuZ29vZ2xldXNlcmNvbnRlbnQuY29tIiwiYXVkIjoiODc2NzMzMTA1MTE2LWkwaGozczUzcWlpbzVrOTVwcnBmbWowaHAwZ21ndG9yLmFwcHMuZ29vZ2xldXNlcmNvbnRlbnQuY29tIiwic3ViIjoiMTA0MTAzMTk1NzYyNTcwODk5MzM3IiwiZW1haWwiOiJsZW50YW4xMDI5QGdtYWlsLmNvbSIsImVtYWlsX3ZlcmlmaWVkIjp0cnVlLCJhdF9oYXNoIjoiOFBhaXRObUJyTmZUQ1V4TjhyVVJCUSIsIm5hbWUiOiJMZW50YW4gVGFuIiwicGljdHVyZSI6Imh0dHBzOi8vbGg0Lmdvb2dsZXVzZXJjb250ZW50LmNvbS8tT05Hd24wUVdUTVUvQUFBQUFBQUFBQUkvQUFBQUFBQUFCVjAvbWZ2Yk9uRWcxMU0vczk2LWMvcGhvdG8uanBnIiwiZ2l2ZW5fbmFtZSI6IkxlbnRhbiIsImZhbWlseV9uYW1lIjoiVGFuIiwibG9jYWxlIjoiZW4iLCJpYXQiOjE1NDA5NjQzNDcsImV4cCI6MTU0MDk2Nzk0NywianRpIjoiYWJmYmIzMjY2ZmJjOWJhN2Q3OGJjODA2YTZkOTQ1Y2E2OTFlYmJkZiJ9.BVZOpaZrX2HHWq1UNjvUft5rhBh54IEO_me9Li_1hCeTXjIaSSJlATxEtCGnYIqTaJ-c_YpN4HUAlS6OqeSXuWbI4QldL90sR7G7W3AwBIajGhleDWi_cifGu06lv3OeTPyIIHzWRZgQfhndNBuOPx723EAnyJ958cu-tRAtDctAIg5tLhc0B9B-XxVrNY8Dz4uExvtHLpU-R-eY9hGxb0pTR5TjaqKx6X91Y62YgECrLMc4Q1eGZ4aXuaE6TTjB_t9bmvvabC-rX8HXCoTkUEpyHlmc4GQzpn0cnuZC7B9sW-9v-we8WTzEIM2Z7DeIb_dewKu8cO3tWUwpUseNtg"
	client := http.DefaultClient
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/tokeninfo?id_token=" + idToken)
	if err != nil {
		fmt.Println(err)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	var body AuthBodyGoogle
	err = json.Unmarshal(b, &body)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf(body.Azp)

}
