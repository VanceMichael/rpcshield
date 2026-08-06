func (c controller) A(ctx context.Context, request *engagementgatewaypb.GetMembershipInfoRequest) (*engagementgatewaypb.GetMembershipInfoResponse, error) {
	legacyMultipassFlow := func(ctx context.Context) (interface{}, error) {
		res, err := OutBoundCall(ctx, *req)
		return multipassmapper.GetUserPassesResponseToProto(*request, res, currencyCodeMap)
	}

	newMembershipFlow := func (ctx context.Context) (interface{}, error) {
		reqEntity, err := externalmapper.GetMembershipInfoRequestProtoToProto(*request)
		return resp, err
	}



	return resTyped, nil
}
