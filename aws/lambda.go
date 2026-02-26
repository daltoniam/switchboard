package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	mcp "github.com/daltoniam/switchboard"
)

func lambdaListFunctions(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &lambda.ListFunctionsInput{}
	if v := argInt32(args, "max_items"); v > 0 {
		input.MaxItems = aws.Int32(v)
	}
	out, err := a.lambdaClient.ListFunctions(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func lambdaGetFunction(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	out, err := a.lambdaClient.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: aws.String(argStr(args, "function_name")),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func lambdaInvoke(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &lambda.InvokeInput{
		FunctionName: aws.String(argStr(args, "function_name")),
	}
	if payload := argStr(args, "payload"); payload != "" {
		input.Payload = []byte(payload)
	}
	if invType := argStr(args, "invocation_type"); invType != "" {
		input.InvocationType = lambdatypes.InvocationType(invType)
	}
	out, err := a.lambdaClient.Invoke(ctx, input)
	if err != nil {
		return errResult(err)
	}

	type invokeResult struct {
		StatusCode    int32  `json:"status_code"`
		FunctionError string `json:"function_error,omitempty"`
		Payload       string `json:"payload"`
	}
	result := invokeResult{
		StatusCode: out.StatusCode,
		Payload:    string(out.Payload),
	}
	if out.FunctionError != nil {
		result.FunctionError = *out.FunctionError
	}
	return jsonResult(result)
}

func lambdaListEventSourceMappings(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &lambda.ListEventSourceMappingsInput{}
	if fn := argStr(args, "function_name"); fn != "" {
		input.FunctionName = aws.String(fn)
	}
	out, err := a.lambdaClient.ListEventSourceMappings(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func lambdaGetFunctionConfiguration(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	out, err := a.lambdaClient.GetFunctionConfiguration(ctx, &lambda.GetFunctionConfigurationInput{
		FunctionName: aws.String(argStr(args, "function_name")),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}
