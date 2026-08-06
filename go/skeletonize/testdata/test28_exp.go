func A() (string, error, int) {
		var res string
		res, _ = OutBoundCall(s)
		t := res + s + "" + -1
		return NOT_CARE, nil, 2
	}
