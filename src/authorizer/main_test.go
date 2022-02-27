package main

import (
	"context"
	"testing"
	"time"

	"capturing-events-apigw-eb/cognito"
	"capturing-events-apigw-eb/config"

	"github.com/aws/aws-lambda-go/events"
)

func TestHandler(t *testing.T) {
	t.Run("Validates the token and returns the policy", func(t *testing.T) {
		token, err := getRandomToken(t)
		if err != nil {
			t.Fatal(err)
		}

		h := NewHandler(HandlerEnvironment{Region: config.Region, UserPoolID: config.UserPoolID, UserPoolClientID: config.UserPoolClientID})
		resp, err := h(context.Background(), events.APIGatewayCustomAuthorizerRequest{AuthorizationToken: token, MethodArn: "ARN"})
		if err != nil {
			t.Fatal(err)
		}

		if resp.PolicyDocument.Statement[0].Effect != "Allow" {
			t.Fatalf("Expected effect to be Allow, got %s", resp.PolicyDocument.Statement[0].Effect)
		}

	})
}

func getRandomToken(t *testing.T) (string, error) {
	t.Helper()

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second*10))
	defer cancel()

	usr, cleanup, err := cognito.NewCognitoUser(ctx)
	if err != nil {
		return "", err
	}

	t.Cleanup(cleanup)

	return usr.AccessToken, err
}
