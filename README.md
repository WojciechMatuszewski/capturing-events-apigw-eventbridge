# Capturing user events with APIGW and EventBridge

Inspired by [this AWS blog post](https://aws.amazon.com/blogs/compute/capturing-client-events-using-amazon-api-gateway-and-amazon-eventbridge/).

Learning goals

- Refresher on [Amazon API Gateway](https://aws.amazon.com/api-gateway/) VTL templates.
- Refresher on [Amazon EventBridge](https://aws.amazon.com/eventbridge/) transformations.
- Contrast this approach of adding newlines to [Amazon Kinesis Data Firehose](https://aws.amazon.com/kinesis/data-firehose/) to the one I've used in [this example repo](https://github.com/WojciechMatuszewski/apigw-access-logs-firehose).

## Deployment

1. `npm install`
2. `npm run bootstrap`
3. `npm run deploy`

## Playing around

1. Use the `npm run token` to get the [Amazon Cognito](https://aws.amazon.com/cognito/) access token. The API is behind an authorizer that validates the Amazon Cognito access tokens.
2. Invoke the API.
3. Observe the logs inside the "debug log group" and the Amazon Kinesis Data Firehose target S3 bucket entries.

## Learnings

- When directly integrating Amazon API Gateway with other AWS services, the `requestOverride` is your friend.

  - Without `requestOverride`, you would not be able to override headers on the `methodRequest`. If you do not override specific headers, the Amazon API Gateway will send a malformed request to another AWS service.

  - For parsing payloads – especially true for services like Amazon EventBridge, which returns an array of statuses for each event you send.

  - **Remember** to map the `methodResponses` property correctly!

- **Remember** that the **updates to the Amazon API Gateway** are **eventually consistent**.

  - It takes time for the change to propagate. Usually, it does not take that long (a couple of seconds at maximum).

- There are **two types of AWS Lambda authorizers** you can apply onto the Amazon API Gateway route. The **request** and the **token** authorizer.

  - The **request** authorizer event contains the **complete API request except the body**. The request body is omitted. I'm not sure why that is the case, maybe for security reasons?

  > A request parameter-based Lambda authorizer (also called a REQUEST authorizer) receives the caller's identity in a combination of headers, query string parameters, stageVariables, and $context variables. ([Source](https://docs.aws.amazon.com/apigateway/latest/developerguide/apigateway-use-lambda-authorizer.html)).

  - The **token** authorizer event contains only the token and the ARN of the invoked method.

- **Using the "TEST" button in Amazon API Gateway (REST) console** will **NOT invoke your authorizer**.

  - This means that all the VTL variables related to the authorizer will not be there.

- Throwing **an error in authorizer** will **result in 500 status code response from APIGW**.

  - It is up to you whether you think the authorizer should or should not throw an error.

  - In my mind, it is reasonable to throw an error in authorizer as such an event will cause the Lambda to fail, most likely paging you as a result.

  - Remember that **authorizer errors are NOT method errors**. This means that **they will NOT be processed by the `requestTemplates`**.

    - To ensure you have control over the error message, use the **[Gateway responses](https://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-gatewayResponse-definition.html)**.

- The **EventBridge `PutEvents` direct API call `Detail` parameter is of type string**, but that string **has to be serializable to JSON object**.

  - [Documentation link](https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_PutEventsRequestEntry.html#eventbridge-Type-PutEventsRequestEntry-Detail).

  - I find this requirement a bit strange.

- **Writing VTL is hard**.

  - Formatting the data to serialize correctly can be a pain, especially if you want to create deep object structures.

  - The `$util.escapeJavaScript` function is there, but, **in my humble opinion**, the documentation is lacking in examples.

  - The **safest and most straightforward approach for me** was to **_base64 encode_ all the problematic values** and then encode them on the way back.

- Something **weird is going on with the `$context` VTL variable** in the context of **mapping templates**.

  - The `#set($escapedCtx = $util.base64Encode($util.escapeJavaScript($context.authorizer)))` returns an empty string.

  - The `#set($escapedCtx = $util.base64Encode($util.escapeJavaScript($context.authorizer.clientId)))` returns the _base64 encoded_ `clientId`.

  - What gives? Is some introspection on the APIGW side performed?

    - Most likely, if you **read the documentation carefully, without assumptions**, you will notice that **the documentation mentions that only `$context.authorizer.PROPERTY` variable is available to the mapping template, not the whole `$context` itself**

    - This would explain the behavior I'm experiencing.

- I love CDK, but I found that the API for creating _EventBridge transformations_ is a bit confusing.

  - Instead of using `InputTransformer` or other CFN properties, they went with a model similar to Step Functions.

  - I had to use an "escape hatch" of sorts (the `bind` method) to achieve what I wanted

    ```ts
    targets: [
      new aws_events_targets.KinesisFirehoseStream(
        firehose.node.defaultChild as cdk.aws_kinesisfirehose.CfnDeliveryStream,
        {
          message: {
            bind: () => ({
              inputPathsMap: {},
              inputTemplate: "<aws.events.event>\n"
            })
          }
        }
      )
    ];
    ```
