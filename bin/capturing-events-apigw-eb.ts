#!/usr/bin/env node
import "source-map-support/register";
import * as cdk from "aws-cdk-lib";
import { CapturingEventsApigwEbStack } from "../lib/capturing-events-apigw-eb-stack";

const app = new cdk.App();
new CapturingEventsApigwEbStack(app, "CapturingEventsApigwEbStack", {
  synthesizer: new cdk.DefaultStackSynthesizer({
    qualifier: "custevents"
  })
});
