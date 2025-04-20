package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.uber.org/zap"
)

// getAllAvailableRegions retrieves all available
// (opted-in and regions where opt-in is not required) regions in the account
func (app *application) getAllAvailableRegions() ([]string, error) {
	app.logger.Infow("all-regions options enabled, getting all available regions")

	in := &ec2.DescribeRegionsInput{
		Filters: []types.Filter{
			{
				Name: aws.String("opt-in-status"),
				Values: []string{
					"opt-in-not-required",
					"opted-in",
				},
			},
		},
	}

	describeRegionsOutput, err := app.ec2Client.DescribeRegions(context.Background(), in)
	if err != nil {
		return nil, err
	}

	optedInRegionsList := []string{}
	for _, region := range describeRegionsOutput.Regions {
		optedInRegionsList = append(optedInRegionsList, *region.RegionName)
	}

	app.logger.Debugw("got all available regions in the account",
		zap.Int("region_count", len(optedInRegionsList)),
	)

	return optedInRegionsList, nil
}
