import {
  aws_apigateway,
  aws_cognito,
  aws_events,
  aws_events_targets,
  aws_iam,
  aws_logs,
  aws_s3,
  Stack,
  StackProps
} from "aws-cdk-lib";
import * as aws_kinesisfirehose from "@aws-cdk/aws-kinesisfirehose-alpha";
import * as aws_kinesisfirehose_destinations from "@aws-cdk/aws-kinesisfirehose-destinations-alpha";
import * as cdk from "aws-cdk-lib";
import * as aws_lambda_go from "@aws-cdk/aws-lambda-go-alpha";
import { Construct } from "constructs";
import { join } from "path";

export class CapturingEventsApigwEbStack extends Stack {
  constructor(scope: Construct, id: string, props?: StackProps) {
    super(scope, id, props);

    new cdk.CfnOutput(this, "Region", {
      value: cdk.Aws.REGION
    });

    const userPool = new aws_cognito.UserPool(this, "UserPool", {
      selfSignUpEnabled: true,
      passwordPolicy: {
        minLength: 6,
        requireLowercase: false,
        requireSymbols: false,
        requireDigits: false,
        requireUppercase: false
      },
      signInAliases: {
        email: true
      }
    });

    new cdk.CfnOutput(this, "UserPoolId", {
      value: userPool.userPoolId
    });

    const userPoolClient = new aws_cognito.UserPoolClient(
      this,
      "UserPoolClient",
      {
        userPool,
        preventUserExistenceErrors: true,
        generateSecret: false,
        authFlows: {
          adminUserPassword: true,
          userSrp: true,
          userPassword: true
        }
      }
    );

    new cdk.CfnOutput(this, "UserPoolClientId", {
      value: userPoolClient.userPoolClientId
    });

    const api = new aws_apigateway.RestApi(this, "API", {
      deployOptions: {
        loggingLevel: aws_apigateway.MethodLoggingLevel.INFO
      }
    });
    api.addGatewayResponse("Default5xx", {
      type: aws_apigateway.ResponseType.DEFAULT_5XX,
      templates: {
        "application/json": `{"message": "Internal Server Error"}`
      }
    });

    const bus = new aws_events.EventBus(this, "EventBus", {});

    const rootGETrole = new aws_iam.Role(this, "RootGETRole", {
      assumedBy: new aws_iam.ServicePrincipal("apigateway.amazonaws.com"),
      description: "Role for the root GET method",
      inlinePolicies: {
        allowEventBridgePutEvents: new aws_iam.PolicyDocument({
          statements: [
            new aws_iam.PolicyStatement({
              effect: aws_iam.Effect.ALLOW,
              actions: ["events:PutEvents"],
              resources: [bus.eventBusArn]
            })
          ]
        })
      }
    });

    const authorizerHandler = new aws_lambda_go.GoFunction(
      this,
      "AuthorizerHandler",
      {
        entry: join(__dirname, "../src/authorizer"),
        environment: {
          REGION: cdk.Aws.REGION,
          USER_POOL_ID: userPool.userPoolId,
          USER_POOL_CLIENT_ID: userPoolClient.userPoolClientId
        }
      }
    );

    const rootGETAuthorizer = new aws_apigateway.TokenAuthorizer(
      this,
      "RootGETAuthorizer",
      {
        handler: authorizerHandler,
        identitySource: "method.request.header.Authorization",
        // Disabled for testing purposes
        resultsCacheTtl: cdk.Duration.minutes(0)
      }
    );

    const rootGET = api.root.addMethod(
      "GET",
      new aws_apigateway.Integration({
        type: aws_apigateway.IntegrationType.AWS,
        integrationHttpMethod: "POST",
        uri: `arn:aws:apigateway:${cdk.Aws.REGION}:events:action/PutEvents`,
        options: {
          requestTemplates: {
            "application/json": `
              #set($context.requestOverride.header.X-Amz-Target = "AWSEvents.PutEvents")
              #set($context.requestOverride.header.Content-Type = "application/x-amz-json-1.1")

              {
                "Entries": [
                  {
                    "Resources": [\"$context.authorizer.clientId\"],
                    "Detail": \"{}\",
                    "DetailType": "detailTypeField",
                    "EventBusName": "${bus.eventBusName}",
                    "Source": "clientevents"
                  }
                ]
              }
            `
          },
          credentialsRole: rootGETrole,
          passthroughBehavior: aws_apigateway.PassthroughBehavior.NEVER,
          integrationResponses: [
            {
              statusCode: "200",
              responseTemplates: {
                "application/json": `
                  #set($failedCount = $input.path('$.FailedEntryCount'))

                  #if ($failedCount > 0)
                    #set($errorMessage = $input.path('$.Entries[0].ErrorMessage'))
                    #set($context.responseOverride.status = 400)
                    {
                      "message": "$errorMessage"
                    }
                  #else
                    {
                      "message": "Event send"
                    }
                  #end
                `
              }
            },
            {
              statusCode: "400",
              selectionPattern: "^4\\d{2}",
              responseTemplates: {
                "application/json": `
                  {
                    "message": "$util.escapeJavaScript($input.json('$'))"
                  }
                `
              }
            },
            {
              /**
               * For example the authorizer panics.
               */
              statusCode: "500",
              selectionPattern: "^5\\d{2}",
              responseTemplates: {
                "application/json": `
                  {
                    "message": "Internal Server Error"
                  }
                `
              }
            }
          ]
        }
      }),
      {
        authorizer: rootGETAuthorizer,
        methodResponses: [
          {
            statusCode: "400"
          },
          {
            statusCode: "200"
          },
          {
            statusCode: "500"
          }
        ]
      }
    );

    const debugLogGroup = new aws_logs.LogGroup(this, "DebugLogGroup", {
      removalPolicy: cdk.RemovalPolicy.DESTROY
    });

    const debugBusRule = new aws_events.Rule(this, "DebugBusRule", {
      description: "Rule to send events to the debug log group",
      eventPattern: {
        source: ["clientevents"]
      },
      eventBus: bus,
      targets: [new aws_events_targets.CloudWatchLogGroup(debugLogGroup)]
    });

    const firehoseBucket = new aws_s3.Bucket(this, "FirehoseBucket", {
      autoDeleteObjects: true,
      removalPolicy: cdk.RemovalPolicy.DESTROY
    });
    const firehose = new aws_kinesisfirehose.DeliveryStream(
      this,
      "DeliveryStream",
      {
        destinations: [
          new aws_kinesisfirehose_destinations.S3Bucket(firehoseBucket, {
            bufferingInterval: cdk.Duration.seconds(60)
          })
        ]
      }
    );

    const firehoseRule = new aws_events.Rule(this, "FirehoseRule", {
      description: "Rule to send events to the firehose",
      eventPattern: {
        source: ["clientevents"]
      },
      eventBus: bus,
      targets: [
        new aws_events_targets.KinesisFirehoseStream(
          firehose.node
            .defaultChild as cdk.aws_kinesisfirehose.CfnDeliveryStream,
          {
            message: {
              bind: () => ({
                inputPathsMap: {},
                inputTemplate: "<aws.events.event>\n"
              })
            }
          }
        )
      ]
    });
  }
}
