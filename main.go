package main

import (
	"context"
	"encoding/csv"
	"flag"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// settings stores the user input arguments when running the program
type settings struct {
	debug          bool
	awsProfileName string
	outputFileName string
	maxWorkers     int
}

// application stores main program global dependencies
type application struct {
	logger       *zap.SugaredLogger
	lambdaClient *lambda.Client
	cwlogsClient *cloudwatchlogs.Client
}

func main() {
	var stg settings
	flag.BoolVar(&stg.debug, "debug", false, "Debug mode. Shows debug logs")
	flag.StringVar(&stg.awsProfileName, "aws-profile", "default", "AWS Profile Name")
	flag.StringVar(&stg.outputFileName, "out-name", "lambda-list.csv", "The name of the output file")
	flag.IntVar(&stg.maxWorkers, "max-workers", 50, "Maximum number of workers")
	flag.Parse()

	logger := createLogger(stg.debug)
	defer logger.Sync()

	logger.Debugf("loading config from aws profile named %q", stg.awsProfileName)
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithSharedConfigProfile(stg.awsProfileName))
	if err != nil {
		logger.Fatalw("error when loading aws profile",
			zap.String("profile_name", stg.awsProfileName),
			zap.Error(err),
		)
	}

	app := &application{
		logger:       logger,
		lambdaClient: lambda.NewFromConfig(cfg),
		cwlogsClient: cloudwatchlogs.NewFromConfig(cfg),
	}

	lambdaFunctionsList, err := app.getAllLambdaFunctionsDetails()
	if err != nil {
		logger.Fatalw("error when listing lambda function details",
			zap.Error(err),
		)
	}

	wg := &sync.WaitGroup{}
	app.getAllLambdaFunctionsLastInvokeTime(lambdaFunctionsList, wg, stg.maxWorkers)
	wg.Wait()

	logger.Infof("writing the output to %q", stg.outputFileName)
	f, err := os.Create(stg.outputFileName)
	if err != nil {
		logger.Errorw("error when creating a file",
			zap.Error(err),
		)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	titles := lambdaFunctionsList[0].getTitleFields()
	err = w.Write(titles)
	if err != nil {
		logger.Errorw("error when writing title",
			zap.Error(err),
		)
	}

	for _, lambdaDetails := range lambdaFunctionsList {
		record := []string{
			lambdaDetails.Name,
			lambdaDetails.Arn,
			lambdaDetails.Description,
			lambdaDetails.LastModified,
			lambdaDetails.IamRole,
			lambdaDetails.Runtime,
			lambdaDetails.LastInvoked,
		}

		err := w.Write(record)
		if err != nil {
			logger.Errorw("error when writing the entry",
				zap.String("function_name", lambdaDetails.Name),
				zap.Error(err),
			)
		}
	}

	logger.Infow("all the function details have been written to the output",
		zap.String("file name", stg.outputFileName),
		zap.Int("number of functions", len(lambdaFunctionsList)),
	)
}

func createLogger(debugMode bool) *zap.SugaredLogger {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	level := zap.NewAtomicLevelAt(zap.InfoLevel)
	if debugMode {
		level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}

	config := zap.Config{
		Level:             level,
		Development:       false,
		DisableCaller:     !debugMode, // disable caller if log level is not debug
		DisableStacktrace: !debugMode, // disable stack trace if log level is not debug
		Encoding:          "console",
		EncoderConfig:     encoderConfig,
		OutputPaths: []string{
			"stdout",
		},
		ErrorOutputPaths: []string{
			"stderr",
		},
	}

	return zap.Must(config.Build()).Sugar()
}
