package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamotypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	mcp "github.com/daltoniam/switchboard"
)

func dynamoListTables(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &dynamodb.ListTablesInput{}
	if v := r.Int32("limit"); v > 0 {
		input.Limit = aws.Int32(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.dynamoClient.ListTables(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func dynamoDescribeTable(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tableName := r.Str("table_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.dynamoClient.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func dynamoGetItem(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	keyStr := r.Str("key")
	tableName := r.Str("table_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var key map[string]dynamotypes.AttributeValue
	if err := unmarshalDynamoJSON(keyStr, &key); err != nil {
		return errResult(err)
	}
	out, err := a.dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key:       key,
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func dynamoPutItem(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	itemStr := r.Str("item")
	tableName := r.Str("table_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var item map[string]dynamotypes.AttributeValue
	if err := unmarshalDynamoJSON(itemStr, &item); err != nil {
		return errResult(err)
	}
	out, err := a.dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func dynamoQuery(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tableName := r.Str("table_name")
	keyCondExpr := r.Str("key_condition_expression")
	exprAttrVals := r.Str("expression_attribute_values")
	exprAttrNames := r.Str("expression_attribute_names")
	indexName := r.Str("index_name")
	limit := r.Int32("limit")
	scanFwd := r.Str("scan_index_forward")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String(keyCondExpr),
	}

	if exprAttrVals != "" {
		var vals map[string]dynamotypes.AttributeValue
		if err := unmarshalDynamoJSON(exprAttrVals, &vals); err != nil {
			return errResult(err)
		}
		input.ExpressionAttributeValues = vals
	}
	if exprAttrNames != "" {
		var names map[string]string
		if err := json.Unmarshal([]byte(exprAttrNames), &names); err != nil {
			return errResult(err)
		}
		input.ExpressionAttributeNames = names
	}
	if indexName != "" {
		input.IndexName = aws.String(indexName)
	}
	if limit > 0 {
		input.Limit = aws.Int32(limit)
	}
	if scanFwd == "false" {
		input.ScanIndexForward = aws.Bool(false)
	}

	out, err := a.dynamoClient.Query(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func dynamoScan(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tableName := r.Str("table_name")
	filterExpr := r.Str("filter_expression")
	exprAttrVals := r.Str("expression_attribute_values")
	exprAttrNames := r.Str("expression_attribute_names")
	limit := r.Int32("limit")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	input := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	}
	if filterExpr != "" {
		input.FilterExpression = aws.String(filterExpr)
	}
	if exprAttrVals != "" {
		var vals map[string]dynamotypes.AttributeValue
		if err := unmarshalDynamoJSON(exprAttrVals, &vals); err != nil {
			return errResult(err)
		}
		input.ExpressionAttributeValues = vals
	}
	if exprAttrNames != "" {
		var names map[string]string
		if err := json.Unmarshal([]byte(exprAttrNames), &names); err != nil {
			return errResult(err)
		}
		input.ExpressionAttributeNames = names
	}
	if limit > 0 {
		input.Limit = aws.Int32(limit)
	}
	out, err := a.dynamoClient.Scan(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func dynamoDeleteItem(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	keyStr := r.Str("key")
	tableName := r.Str("table_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var key map[string]dynamotypes.AttributeValue
	if err := unmarshalDynamoJSON(keyStr, &key); err != nil {
		return errResult(err)
	}
	out, err := a.dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key:       key,
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func unmarshalDynamoJSON(s string, out *map[string]dynamotypes.AttributeValue) error {
	var raw map[string]map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return err
	}
	result := make(map[string]dynamotypes.AttributeValue, len(raw))
	for k, typeVal := range raw {
		for typeName, val := range typeVal {
			av, err := unmarshalAttributeValue(typeName, val)
			if err != nil {
				return fmt.Errorf("attribute %q: %w", k, err)
			}
			result[k] = av
		}
	}
	*out = result
	return nil
}

func unmarshalAttributeValue(typeName string, val json.RawMessage) (dynamotypes.AttributeValue, error) {
	switch typeName {
	case "S":
		var sv string
		if err := json.Unmarshal(val, &sv); err != nil {
			return nil, err
		}
		return &dynamotypes.AttributeValueMemberS{Value: sv}, nil
	case "N":
		var nv string
		if err := json.Unmarshal(val, &nv); err != nil {
			return nil, err
		}
		return &dynamotypes.AttributeValueMemberN{Value: nv}, nil
	case "BOOL":
		var bv bool
		if err := json.Unmarshal(val, &bv); err != nil {
			return nil, err
		}
		return &dynamotypes.AttributeValueMemberBOOL{Value: bv}, nil
	case "NULL":
		return &dynamotypes.AttributeValueMemberNULL{Value: true}, nil
	case "B":
		var b []byte
		if err := json.Unmarshal(val, &b); err != nil {
			return nil, err
		}
		return &dynamotypes.AttributeValueMemberB{Value: b}, nil
	case "SS":
		var ss []string
		if err := json.Unmarshal(val, &ss); err != nil {
			return nil, err
		}
		return &dynamotypes.AttributeValueMemberSS{Value: ss}, nil
	case "NS":
		var ns []string
		if err := json.Unmarshal(val, &ns); err != nil {
			return nil, err
		}
		return &dynamotypes.AttributeValueMemberNS{Value: ns}, nil
	case "BS":
		var bs [][]byte
		if err := json.Unmarshal(val, &bs); err != nil {
			return nil, err
		}
		return &dynamotypes.AttributeValueMemberBS{Value: bs}, nil
	case "L":
		var rawList []map[string]json.RawMessage
		if err := json.Unmarshal(val, &rawList); err != nil {
			return nil, err
		}
		list := make([]dynamotypes.AttributeValue, 0, len(rawList))
		for _, item := range rawList {
			for tn, tv := range item {
				av, err := unmarshalAttributeValue(tn, tv)
				if err != nil {
					return nil, err
				}
				list = append(list, av)
			}
		}
		return &dynamotypes.AttributeValueMemberL{Value: list}, nil
	case "M":
		var rawMap map[string]map[string]json.RawMessage
		if err := json.Unmarshal(val, &rawMap); err != nil {
			return nil, err
		}
		m := make(map[string]dynamotypes.AttributeValue, len(rawMap))
		for mk, mv := range rawMap {
			for tn, tv := range mv {
				av, err := unmarshalAttributeValue(tn, tv)
				if err != nil {
					return nil, err
				}
				m[mk] = av
			}
		}
		return &dynamotypes.AttributeValueMemberM{Value: m}, nil
	default:
		return nil, fmt.Errorf("unsupported DynamoDB type: %s", typeName)
	}
}
