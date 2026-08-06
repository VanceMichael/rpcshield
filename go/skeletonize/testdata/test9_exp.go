func (c controller) A(ctx context.Context, request *engagementgatewaypb.GetMembershipInfoRequest) (*engagementgatewaypb.GetMembershipInfoResponse, error) {
	legacyMultipassFlow := func(ctx context.Context) (interface{}, error) {
		res, err := OutBoundCall(ctx, *req)
		if err != nil {
			logger.FromContext(ctx).With(zap.Reflect("Multipass::GetUserPasses request", req)).
				Warn("GetMembershipInfoNotBatch - Multipass::GetUserPasses call failed", zap.Error(err))
			return nil, err
		}

		return multipassmapper.GetUserPassesResponseToProto(*request, res, currencyCodeMap)
	}

	return NOT_CARE, nil
}
