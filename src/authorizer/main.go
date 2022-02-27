package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
)

func main() {
	region := mustGetEnv("REGION")
	userPoolID := mustGetEnv("USER_POOL_ID")
	userPoolClientID := mustGetEnv("USER_POOL_CLIENT_ID")

	h := NewHandler(HandlerEnvironment{Region: region, UserPoolID: userPoolID, UserPoolClientID: userPoolClientID})
	lambda.Start(h)
}

type Handler func(ctx context.Context,
	event events.APIGatewayCustomAuthorizerRequest) (events.APIGatewayCustomAuthorizerResponse, error)

type HandlerEnvironment struct {
	Region           string
	UserPoolID       string
	UserPoolClientID string
}

func NewHandler(environment HandlerEnvironment) Handler {
	return func(ctx context.Context,
		event events.APIGatewayCustomAuthorizerRequest) (events.APIGatewayCustomAuthorizerResponse, error) {

		fmt.Println("Event", event)

		issuer := fmt.Sprintf(
			"https://cognito-idp.%v.amazonaws.com/%v",
			environment.Region, environment.UserPoolID)

		keySetURL := fmt.Sprintf(
			"https://cognito-idp.%v.amazonaws.com/%v/.well-known/jwks.json",
			environment.Region, environment.UserPoolID)

		keyset, err := jwk.Fetch(ctx, keySetURL)
		if err != nil {
			fmt.Println("Failed to fetch the keyset", err.Error())
			return events.APIGatewayCustomAuthorizerResponse{}, err
		}

		token := event.AuthorizationToken
		parsedToken, err := jwt.Parse(
			[]byte(token),
			jwt.WithKeySet(keyset),
			jwt.WithValidate(true),
			jwt.WithIssuer(issuer),
			jwt.WithClaimValue("client_id", environment.UserPoolClientID),
			jwt.WithClaimValue("token_use", "access"),
		)
		if err != nil {
			fmt.Println("Failed to parse the token", err.Error())
			return events.APIGatewayCustomAuthorizerResponse{}, err
		}

		username, found := parsedToken.Get("username")
		if !found {
			fmt.Println("Failed to find username in the token")
			return events.APIGatewayCustomAuthorizerResponse{}, err
		}

		fmt.Println("Returning with a successful response")
		return events.APIGatewayCustomAuthorizerResponse{
			PrincipalID: username.(string),
			Context: map[string]interface{}{
				"clientId": username.(string),
			},
			PolicyDocument: events.APIGatewayCustomAuthorizerPolicy{
				Version: "2012-10-17",
				Statement: []events.IAMPolicyStatement{
					{
						Effect:   "Allow",
						Action:   []string{"execute-api:Invoke"},
						Resource: []string{event.MethodArn},
					},
				},
			},
		}, nil

	}
}

func mustGetEnv(key string) string {
	v, present := os.LookupEnv(key)
	if !present {
		panic(fmt.Sprintf("missing environment variable: %v", key))
	}

	return v
}
