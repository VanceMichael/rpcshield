func (c controller) A(ctx context.Context, request *engagementgatewaypb.GetMembershipInfoRequest) (*engagementgatewaypb.GetMembershipInfoResponse, error) {
	res, err := OutBoundCall(ctx, *req)
	newMembershipFlow := func(ctx context.Context) (interface{}, error) {
		reqEntity, err := externalmapper.GetMembershipInfoRequestProtoToProto(res)
		return resp, err
	}


	return resTyped, nil
}
