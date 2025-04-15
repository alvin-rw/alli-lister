package main

import "reflect"

// lambdaFunctionDetails holds the details of the lambda function that will be printed
// `title` tag is the title of the column of the resulting CSV file
type lambdaFunctionDetails struct {
	Name         string `title:"Function Name"`
	Arn          string `title:"Function ARN"`
	Description  string `title:"Function Description"`
	LastModified string `title:"Last Modified"`
	IamRole      string `title:"IAM Role"`
	Runtime      string `title:"Runtime"`
	LastInvoked  string `title:"Last Invoked"`
}

// getTitleFields will return a list of strings that is populated by the struct title tag
func (l lambdaFunctionDetails) getTitleFields() []string {
	var titles []string

	value := reflect.ValueOf(l)
	for i := range value.NumField() {
		title := value.Type().Field(i).Tag.Get("title")
		titles = append(titles, title)
	}

	return titles
}
