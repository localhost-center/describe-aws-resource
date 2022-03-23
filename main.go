package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/olekukonko/tablewriter"
)

var TraceableRegions = [...]string{"us-east-1", "us-east-2", "us-west-1", "us-west-2", "ca-central-1", "eu-central-1", "eu-west-1", "eu-west-2", "eu-west-3", "eu-north-1", "ap-northeast-1", "ap-northeast-2", "ap-northeast-3", "ap-southeast-1", "ap-southeast-2", "ap-south-1", "sa-east-1"}

type SingleResource struct {
	Region  *string
	Service *string
	Product *string
	Details *string
	ID      *string
	ARN     *string
}

func PrettyPrintResources(resources []*SingleResource) {
	var data [][]string

	for _, r := range resources {
		row := []string{
			DerefNilPointerStrings(r.Region),
			DerefNilPointerStrings(r.Service),
			DerefNilPointerStrings(r.Product),
			DerefNilPointerStrings(r.ID),
		}
		data = append(data, row)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Region", "Service", "Product", "ID"})
	table.SetBorder(true)
	table.AppendBulk(data)
	table.Render()
}

func ServiceNameFromARN(arn *string) *string {
	shortArn := strings.Replace(*arn, "arn:aws:", "", -1)
	sliced := strings.Split(shortArn, ":")
	return &sliced[0]
}

func ShortArn(arn *string) string {
	slicedArn := strings.Split(*arn, ":")
	shortArn := slicedArn[5:]
	return strings.Join(shortArn, "/")
}

type awsEC2 string

type awsECS string

type awsGeneric string

func (aws *awsGeneric) ConverToResource(shortArn, svc, rgn *string) *SingleResource {
	return &SingleResource{ARN: shortArn, Region: rgn, Service: svc, ID: shortArn}
}

func (aws *awsEC2) ConvertToResource(shortArn, svc, rgn *string) *SingleResource {

	s := strings.Split(*shortArn, "/")
	return &SingleResource{ARN: shortArn, Region: rgn, Service: svc, Product: &s[0], ID: &s[1]}
}

func (aws *awsECS) ConvertToResource(shortArn, svc, rgn *string) *SingleResource {

	s := strings.Split(*shortArn, "/")
	return &SingleResource{ARN: shortArn, Region: rgn, Service: svc, Product: &s[0], ID: &s[1]}
}

func ConvertArnToSingleResource(arn, svc, rgn *string) *SingleResource {
	shortArn := ShortArn(arn)

	switch *svc {
	case "ec2":
		res := awsEC2(*svc)
		return res.ConvertToResource(&shortArn, svc, rgn)
	case "ecs":
		res := awsECS(*svc)
		return res.ConvertToResource(&shortArn, svc, rgn)
	default:
		res := awsGeneric(*svc)
		return res.ConverToResource(&shortArn, svc, rgn)
	}
}

func DerefNilPointerStrings(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func main() {
	var resources []*SingleResource

	for _, region := range TraceableRegions {

		cfg := aws.Config{Region: aws.String(region)}
		s := session.Must(session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
			Config:            cfg,
		}))

		r := resourcegroupstaggingapi.New(s)

		var paginationToken string = ""
		var in *resourcegroupstaggingapi.GetResourcesInput
		var out *resourcegroupstaggingapi.GetResourcesOutput
		var err error

		for {
			if len(paginationToken) == 0 {
				in = &resourcegroupstaggingapi.GetResourcesInput{
					ResourcesPerPage: aws.Int64(50),
				}
				out, err = r.GetResources(in)
				if err != nil {
					fmt.Println(err)
				}
			} else {
				in = &resourcegroupstaggingapi.GetResourcesInput{
					ResourcesPerPage: aws.Int64(50),
					PaginationToken:  &paginationToken,
				}
			}

			out, err = r.GetResources(in)
			if err != nil {
				fmt.Println(err)
			}

			for _, resource := range out.ResourceTagMappingList {
				svc := ServiceNameFromARN(resource.ResourceARN)
				rgn := region

				resources = append(resources, ConvertArnToSingleResource(resource.ResourceARN, svc, &rgn))
			}

			paginationToken = *out.PaginationToken
			if *out.PaginationToken == "" {
				break
			}
		}
	}

	PrettyPrintResources(resources)
}
