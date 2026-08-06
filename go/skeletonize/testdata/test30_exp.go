func A() (string, error) {
		var res string
		res, err = OutBoundCall(s)
		t := res + s + "" + -1
		return NOT_CARE, func(ctx context.Context) (interface{}, error) {
			reqEntity, err := externalmapper.GetMembershipInfoRequestProtoToProto(res)
			return resp, err
			}
	}
