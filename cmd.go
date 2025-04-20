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

// job contains the required information for a worker goroutines
// to be able to query the Lambda function last invocation time
// and writes the result back to the Lambda function slice
type job struct {
	functionName string
	region       string
	index        int
}

const (
	lambdaLogGroupPrefix = "/aws/lambda/"

	cloudWatchLogGroupDoesNotExistErrorMessage = "The specified log group does not exist"
)

// getAllLambdaFunctionsDetails returns slice containing the details of all
// Lambda functions in the region specified by regions parameter
func (app *application) getAllLambdaFunctionsDetails() ([]lambdaFunction, error) {
	app.logger.Info("getting function details for lambda functions")

	var lambdaFunctionsList []lambdaFunction
	in := &lambda.ListFunctionsInput{}

	for _, lambdaClient := range app.lambdaClients {
		app.logger.Debugw("getting Lambda functions",
			zap.String("current_region", lambdaClient.Options().Region),
		)

		for {
			out, err := lambdaClient.ListFunctions(context.Background(), in)
			if err != nil {
				return nil, err
			}

			for _, functionDetail := range out.Functions {
				f := lambdaFunction{
					Name:         *functionDetail.FunctionName,
					Region:       lambdaClient.Options().Region,
					Arn:          *functionDetail.FunctionArn,
					Description:  *functionDetail.Description,
					LastModified: *functionDetail.LastModified,
					IamRole:      *functionDetail.Role,
					Runtime:      string(functionDetail.Runtime),
				}

				lambdaFunctionsList = append(lambdaFunctionsList, f)
			}

			if out.NextMarker != nil {
				in.Marker = out.NextMarker
				continue
			} else {
				break
			}
		}
	}

	app.logger.Infow("got all lambda function details",
		zap.Int("function_count", len(lambdaFunctionsList)),
	)

	return lambdaFunctionsList, nil
}

// getAllLambdaFunctionsLastInvokeTime wraps getLambdaFunctionLastInvokeTime and invoke them concurrently in the background.
func (app *application) getAllLambdaFunctionsLastInvokeTime(lambdaFunctionsList []lambdaFunction, wg *sync.WaitGroup, maxWorkers int) {
	app.logger.Info("getting last invoke time for all lambda functions")

	// jobs channel is used to limit the number of workers goroutines
	// by limiting the amount of jobs that can be stored in the channel
	jobs := make(chan job, maxWorkers)

	for i, lambdaDetails := range lambdaFunctionsList {
		currentJob := job{
			functionName: lambdaDetails.Name,
			region:       lambdaDetails.Region,
			index:        i,
		}

		jobs <- currentJob
	}

	for range maxWorkers {
		wg.Add(1)
		go app.getLambdaFunctionLastInvokeTime(jobs, lambdaFunctionsList, wg)
	}

	close(jobs)
}

// getLambdaFunctionLastInvokeTime queries CloudWatch logs to retrieve the latest log timestamp
// of the Lambda function which name is obtained from jobs channel
// and write the output in the lambdaFunctionsList slice. If there's an error when describing the
// CloudWatch log group and log stream, the resulting last invocation timestamp is "-"
func (app *application) getLambdaFunctionLastInvokeTime(jobs <-chan job, lambdaFunctionsList []lambdaFunction, wg *sync.WaitGroup) {
	defer wg.Done()

	for currentJob := range jobs {
		logGroupName := fmt.Sprintf("%s%s", lambdaLogGroupPrefix, currentJob.functionName)

		input := &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: aws.String(logGroupName),
			Descending:   aws.Bool(false),
			Limit:        aws.Int32(1),
			OrderBy:      types.OrderByLastEventTime,
		}

		// TODO: check concurrency logic and make sure that the describe is working as intended
		// TODO: make sure that the region used is the same for describing lambda function and describing cloudwatch logs

		cwLogsClient := cloudwatchlogs.NewFromConfig(*app.cfg, func(o *cloudwatchlogs.Options) {
			o.Region = currentJob.region
		})

		out, err := cwLogsClient.DescribeLogStreams(context.Background(), input)
		if err != nil {
			var oe *smithy.OperationError
			if errors.As(err, &oe) {
				if oe.Operation() == "DescribeLogStreams" && strings.Contains(oe.Unwrap().Error(), cloudWatchLogGroupDoesNotExistErrorMessage) {
					app.logger.Debugw("CloudWatch log group does not exist for lambda function",
						zap.String("function_name", currentJob.functionName),
					)

					lambdaFunctionsList[currentJob.index].LastInvoked = "-"
				}
			} else {
				app.logger.Debugw("error when describing log stream",
					zap.String("log group name", logGroupName),
					zap.Error(err),
				)
			}
		} else if len(out.LogStreams) == 0 {
			app.logger.Debugw("no log stream exists for lambda function",
				zap.String("function_name", currentJob.functionName),
			)

			lambdaFunctionsList[currentJob.index].LastInvoked = "-"
		} else {
			if out != nil && out.LogStreams != nil && out.LogStreams[0].LastEventTimestamp != nil {
				lastEventTimestampInSeconds := *out.LogStreams[0].LastEventTimestamp / 1000
				t := time.Unix(lastEventTimestampInSeconds, 0)

				lambdaFunctionsList[currentJob.index].LastInvoked = t.Format("2006-01-02T15:04:05-07:00")
				app.logger.Debugw("last invoke time info",
					zap.Int64("*out.LogStreams[0].LastEventTimestamp", *out.LogStreams[0].LastEventTimestamp/1000),
					zap.Int64("lastEventTimestampInSeconds", lastEventTimestampInSeconds),
					zap.String("formatted time", t.Format("2006-01-02T15:04:05-07:00")),
					zap.String("lambdaFunctionsList[index].lastInvoked", lambdaFunctionsList[currentJob.index].LastInvoked),
				)
			}
		}
	}
}
