package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// settings stores the user input arguments when running the program
type settings struct {
	debug          bool
	awsProfileName string
	getAllRegions  bool
	outputFileName string
	maxWorkers     int
}

// application stores main program global dependencies
type application struct {
	logger        *zap.SugaredLogger
	cfg           *aws.Config
	ec2Client     *ec2.Client
	lambdaClients []*lambda.Client
}

func main() {
	var stg settings
	flag.BoolVar(&stg.debug, "debug", false, "Debug mode. Shows debug logs")
	flag.StringVar(&stg.awsProfileName, "aws-profile", "default", "AWS Profile Name")
	flag.BoolVar(&stg.getAllRegions, "all-regions", false, "Whether to get data from all AWS Regions")
	flag.StringVar(&stg.outputFileName, "output-file-name", "", "The name of the output file. If not provided, the resulting file name will be [timestamp].csv")
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

	app, err := initializeApplication(logger, cfg, stg.getAllRegions)
	if err != nil {
		logger.Fatalw("error when initializing application struct",
			zap.Error(err),
		)
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

	fileName := getFileName(stg.outputFileName)

	logger.Infof("writing the output to %q", fileName)
	f, err := os.Create(fileName)
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
			lambdaDetails.Region,
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
		zap.String("file name", fileName),
		zap.Int("number of functions", len(lambdaFunctionsList)),
	)
}

// createLogger creates zap.SugaredLogger with debug or info logging level
// depending on the input
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

// initializeApplication creates application struct with logger and AWS Service Clients (ec2Client, lambdaClients, and cwLogsClients).
//
// lambdaClients and cwLogsClients are created based on the number of regions.
// If getAllRegions is set to true, it will populate the application struct with clients for all AWS Regions
func initializeApplication(logger *zap.SugaredLogger, cfg aws.Config, getAllRegions bool) (*application, error) {
	logger.Debug("initializing application struct")

	app := &application{
		logger:    logger,
		cfg:       &cfg,
		ec2Client: ec2.NewFromConfig(cfg),
	}

	logger.Debugw("getting chosen regions",
		zap.Bool("all_regions", getAllRegions),
	)
	// get regions list based on the chosen parameters
	regions := []string{}
	if getAllRegions {
		allOptedInRegions, err := app.getAllOptedInRegions()
		if err != nil {
			app.logger.Fatalf("error when listing all available regions",
				zap.Error(err),
			)
		}

		regions = allOptedInRegions
	} else {
		// if no specified region is chosen, use AWS CLI default region
		regions = append(regions, cfg.Region)
	}

	// lambdaClients will hold all the service clients from all chosen regions.
	// This will be used to query the AWS Service
	lambdaClients := []*lambda.Client{}

	logger.Debug("initializing service clients for chosen regions")
	// Create AWS service clients for all chosen region and put it in the application struct
	for _, region := range regions {
		lambdaClient := lambda.NewFromConfig(cfg, func(o *lambda.Options) {
			o.Region = region
		})
		lambdaClients = append(lambdaClients, lambdaClient)
	}
	logger.Debug("service clients retrieved")

	app.lambdaClients = lambdaClients

	return app, nil
}

// getFileName generates file name based on the user input. If the user does not input a file name,
// it returns filename with format [timestamp].csv, e.g. 1744990200.csv
func getFileName(inputFileName string) string {
	if inputFileName == "" {
		return fmt.Sprintf("%d.csv", time.Now().Unix())
	} else {
		return inputFileName
	}
}
