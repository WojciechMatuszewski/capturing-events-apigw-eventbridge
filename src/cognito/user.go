package cognito

import (
	"context"
	"fmt"
	"time"

	"capturing-events-apigw-eb/config"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	cognitoidentityprovidertypes "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/google/uuid"
)

type CognitoUser struct {
	Username      string
	Email         string
	AccessToken   string
	IdentityToken string
}

func NewCognitoUser(ctx context.Context) (CognitoUser, func(), error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		panic(fmt.Sprintf("Could not initialize the config: %v", err))
	}
	identityProvider := cognitoidentityprovider.NewFromConfig(cfg)

	email := fmt.Sprintf("%s@test.pl", uuid.New().String())
	signUpResp, err := identityProvider.SignUp(ctx, &cognitoidentityprovider.SignUpInput{
		ClientId: aws.String(config.UserPoolClientID),
		Username: aws.String(email),
		Password: aws.String("test-password"),
	})
	if err != nil {
		return CognitoUser{}, nil, fmt.Errorf("could not sign up the user: %v", err)
	}
	cleanup := func() {
		deleteCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second*10))
		defer cancel()

		_, err = identityProvider.AdminDeleteUser(deleteCtx, &cognitoidentityprovider.AdminDeleteUserInput{
			UserPoolId: aws.String(config.UserPoolID),
			Username:   signUpResp.UserSub,
		})
		if err != nil {
			panic(fmt.Sprintf("Could not delete the user: %v", err))
		}
	}

	_, err = identityProvider.AdminConfirmSignUp(ctx, &cognitoidentityprovider.AdminConfirmSignUpInput{
		UserPoolId: aws.String(config.UserPoolID),
		Username:   aws.String(*signUpResp.UserSub),
	})
	if err != nil {
		cleanup()
		return CognitoUser{}, nil, fmt.Errorf("could not confirm the user: %v", err)
	}

	authResp, err := identityProvider.InitiateAuth(ctx, &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow: cognitoidentityprovidertypes.AuthFlowTypeUserPasswordAuth,
		ClientId: aws.String(config.UserPoolClientID),
		AuthParameters: map[string]string{
			"USERNAME": *signUpResp.UserSub,
			"PASSWORD": "test-password",
		},
	})
	if err != nil {
		cleanup()
		return CognitoUser{}, nil, fmt.Errorf("could not initiate the auth: %v", err)
	}

	return CognitoUser{
		Username:      *signUpResp.UserSub,
		Email:         email,
		AccessToken:   *authResp.AuthenticationResult.AccessToken,
		IdentityToken: *authResp.AuthenticationResult.IdToken,
	}, cleanup, nil
}
