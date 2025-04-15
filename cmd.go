package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/smithy-go"
	"go.uber.org/zap"
)

const (
	lambdaLogGroupPrefix = "/aws/lambda/"

	cloudWatchLogGroupDoesNotExistErrorMessage = "The specified log group does not exist"
)

// getAllLambdaFunctionsDetailsList lists all Lambda functions in the AWS Account in the current region
func (app *application) getAllLambdaFunctionsDetailsList() ([]lambdaFunctionDetails, error) {
	app.logger.Info("getting function details for all lambda functions")

	in := &lambda.ListFunctionsInput{}
	var lambdaFunctionsDetailsList []lambdaFunctionDetails

	for {
		out, err := app.lambdaClient.ListFunctions(context.Background(), in)
		if err != nil {
			return nil, err
		}

		for _, functionDetail := range out.Functions {
			f := lambdaFunctionDetails{
				Name:         *functionDetail.FunctionName,
				Arn:          *functionDetail.FunctionArn,
				Description:  *functionDetail.Description,
				LastModified: *functionDetail.LastModified,
				IamRole:      *functionDetail.Role,
				Runtime:      string(functionDetail.Runtime),
			}

			lambdaFunctionsDetailsList = append(lambdaFunctionsDetailsList, f)
		}

		if out.NextMarker != nil {
			in.Marker = out.NextMarker
			continue
		} else {
			break
		}
	}

	app.logger.Infow("got all lambda function details",
		zap.Int("function_count", len(lambdaFunctionsDetailsList)),
	)

	return lambdaFunctionsDetailsList, nil
}

func (app *application) getAllLambdaFunctionsLastInvokeTimeBackground(outputlist []lambdaFunctionDetails, wg *sync.WaitGroup) {
	app.logger.Info("getting last invoke time for all lambda functions")

	for i, lambdaDetails := range outputlist {
		wg.Add(1)
		go app.getLambdaFunctionLastInvokeTimeBackground(lambdaDetails.Name, i, outputlist, wg)
	}
}

func (app *application) getLambdaFunctionLastInvokeTimeBackground(functionName string, index int, outputList []lambdaFunctionDetails, wg *sync.WaitGroup) {
	defer wg.Done()

	logGroupName := fmt.Sprintf("%s%s", lambdaLogGroupPrefix, functionName)

	input := &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: aws.String(logGroupName),
		Descending:   aws.Bool(false),
		Limit:        aws.Int32(1),
		OrderBy:      types.OrderByLastEventTime,
	}

	out, err := app.cwlogsClient.DescribeLogStreams(context.Background(), input)
	if err != nil {
		var oe *smithy.OperationError
		if errors.As(err, &oe) {
			if oe.Operation() == "DescribeLogStreams" && strings.Contains(oe.Unwrap().Error(), cloudWatchLogGroupDoesNotExistErrorMessage) {
				app.logger.Debugw("CloudWatch log group does not exist for lambda function",
					zap.String("function_name", functionName),
				)

				outputList[index].LastInvoked = "-"
			}
		} else {
			app.logger.Debugw("error when describing log stream",
				zap.String("log group name", logGroupName),
				zap.Error(err),
			)
		}
	} else if len(out.LogStreams) == 0 {
		app.logger.Debugw("no log stream exists for lambda function",
			zap.String("function_name", functionName),
		)

		outputList[index].LastInvoked = "-"
	} else {
		if out != nil && out.LogStreams != nil && out.LogStreams[0].LastEventTimestamp != nil {
			lastEventTimestampInSeconds := *out.LogStreams[0].LastEventTimestamp / 1000
			t := time.Unix(lastEventTimestampInSeconds, 0)

			outputList[index].LastInvoked = t.Format("2006-01-02T15:04:05-07:00")
			app.logger.Debugw("last invoke time info",
				zap.Int64("*out.LogStreams[0].LastEventTimestamp", *out.LogStreams[0].LastEventTimestamp/1000),
				zap.Int64("lastEventTimestampInSeconds", lastEventTimestampInSeconds),
				zap.String("formatted time", t.Format("2006-01-02T15:04:05-07:00")),
				zap.String("outputList[index].lastInvoked", outputList[index].LastInvoked),
			)
		}
	}
}
