package main

import (
	"capturing-events-apigw-eb/cognito"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx := context.Background()
	usr, cleanup, err := cognito.NewCognitoUser(ctx)
	if err != nil {
		fmt.Println("ERROR", err)
		panic(fmt.Sprintf("Could not create the user: %v", err))
	}

	ctx, stop := signal.NotifyContext(
		ctx,
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	blockChan := make(chan struct{}, 1)
	go func() {
		<-ctx.Done()

		defer func() {
			stop()
			close(blockChan)
		}()

		fmt.Println("Stopping...")

		cleanup()
	}()

	fmt.Println(usr.AccessToken)

	<-blockChan

	os.Exit(0)
}
