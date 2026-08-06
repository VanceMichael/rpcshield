func A() (string, error) {
		var res string
		res, _ = OutBoundCall(s)
		t := res + s + "" + -1
		return NOT_CARE, func (ctx context.Context) (interface{}, error) {
			return NOT_CARE,NOT_CARE
			}
	}
