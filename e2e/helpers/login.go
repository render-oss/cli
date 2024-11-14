package helpers

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var urlRegex = regexp.MustCompile(`device-authorization/([\w-]*)`)
var successRegex = regexp.MustCompile(`Success`)

type Result struct {
	Data struct {
		SignIn struct {
			IDToken string `json:"idToken"`
		} `json:"signIn"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func Login() error {
	// Get Render Host
	host := os.Getenv("RENDER_HOST")
	if host == "" {
		host = "https://api.render.com/v1"
	}

	// Get email
	email := os.Getenv("RENDER_EMAIL")

	// Get Password
	password := os.Getenv("RENDER_PASSWORD")

	result := Result{}
	client := NewGraphQLClient(fmt.Sprintf("%s/graphql", strings.Replace(host, "/v1", "", 1)))
	err := client.Execute(`
		mutation signIn($email: String!, $password: String!) {
			signIn(email: $email, password: $password) {
				idToken
			}
		}
	`, map[string]interface{}{
		"email":    email,
		"password": password,
	}, &result)

	if err != nil {
		return err
	}

	for _, gqlError := range result.Errors {
		return fmt.Errorf(gqlError.Message)
	}

	token := result.Data.SignIn.IDToken

	reader := RunCommand([]string{"login", "-o=json"})

	var urlPath string
	scan := bufio.NewScanner(reader)
	for scan.Scan() {
		line := scan.Text()
		if urlRegex.MatchString(line) {
			matches := urlRegex.FindStringSubmatch(line)
			urlPath = fmt.Sprintf("/device-authorization/%s", matches[1])
			break
		}
	}

	deviceAuthURL, err := url.Parse(fmt.Sprintf("%s%s", host, urlPath))
	if err != nil {
		return err
	}

	const COUNT = 3

	for i := 0; i <= COUNT; i++ {
		httpClient := &http.Client{}
		res, err := httpClient.Do(&http.Request{
			Header: map[string][]string{
				"Authorization": {fmt.Sprintf("Bearer %s", token)},
			},
			Method: "POST",
			URL:    deviceAuthURL,
			Body:   io.NopCloser(strings.NewReader(`{"status": "approved"}`)),
		})

		if err == nil && res.StatusCode == http.StatusOK {
			break
		}

		if i == COUNT {
			return err
		}

		time.Sleep(5 * time.Second)
		continue
	}

	for scan.Scan() {
		line := scan.Text()
		if successRegex.MatchString(line) {
			break
		}
	}

	return nil
}
