package main

import "reflect"

// lambdaFunction contains the details of the lambda function that will be printed
// `title` tag is the title of the column of the resulting CSV file
type lambdaFunction struct {
	Name         string `title:"Function Name"`
	Arn          string `title:"Function ARN"`
	Description  string `title:"Function Description"`
	LastModified string `title:"Last Modified"`
	IamRole      string `title:"IAM Role"`
	Runtime      string `title:"Runtime"`
	LastInvoked  string `title:"Last Invoked"`
}

// getTitleFields will return a list of strings that is populated by the struct title tag.
// This is done to make sure that if the struct fields change in the future, the title fields are still accurate
func (l lambdaFunction) getTitleFields() []string {
	var titles []string

	value := reflect.ValueOf(l)
	for i := range value.NumField() {
		title := value.Type().Field(i).Tag.Get("title")
		titles = append(titles, title)
	}

	return titles
}
