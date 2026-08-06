package main

import "fmt"

func OutBoundCall() (string, error) {
	fmt.Println("OutBoundCall")
	return "outbound", nil
}

func (c controller) A(ctx context.Context, request *engagementgatewaypb.GetMembershipInfoRequest) (*engagementgatewaypb.GetMembershipInfoResponse, error) {
	res, err := OutBoundCall(ctx, *req)
	newMembershipFlow := func(ctx context.Context) (interface{}, error) {
		reqEntity, err := externalmapper.GetMembershipInfoRequestProtoToProto(res)
		timer := metrics.FromContext(ctx).Tagged(map[string]string{
			"scenario":           "eg::getmembershipinforesp",
			"num_of_memberships": strconv.Itoa(len(respEntity.GetUserMembershipInfo())),
		}).Timer(_mapperLatencyMetric).Start()

		return resp, err
	}

	return resTyped, nil
}

func main() {
	word := A()
	fmt.Println(word)
}
