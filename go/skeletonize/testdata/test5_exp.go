func A() (string, error) {
		var res string
		if NOT_CARE {
			res, err := OutBoundCall(s)
			if res != nil {
				C(err)
			} else {
				res = res + "world"
			}
			return res, err
		} else {
			B(res)
		}
		return NOT_CARE, nil
	}
