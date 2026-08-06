package main

import "fmt"

func OutBoundCall() (string, error) {
	fmt.Println("OutBoundCall")
	return "outbound", nil
}

func (c controller) A(ctx context.Context, request *engagementgatewaypb.GetMembershipInfoRequest) (*engagementgatewaypb.GetMembershipInfoResponse, error) {
	legacyMultipassFlow := func(ctx context.Context) (interface{}, error) {
		req := multipassmapper.GetMembershipInfoRequestProtoToThrift(request)
		res, err := OutBoundCall(ctx, *req)
		if err != nil {
			logger.FromContext(ctx).With(zap.Reflect("Multipass::GetUserPasses request", req)).
				Warn("GetMembershipInfoNotBatch - Multipass::GetUserPasses call failed", zap.Error(err))
			metrics.FromContext(ctx).Counter(_multipassGetUserPassesFailInNotBatchCall).Inc(1)
			return nil, err
		}

		return multipassmapper.GetUserPassesResponseToProto(*request, res, currencyCodeMap)
	}

	return resTyped, nil
}

func main() {
	word := A()
	fmt.Println(word)
}
