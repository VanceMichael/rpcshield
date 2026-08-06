func A() (string, error) {
		var res string
		if NOT_CARE {
		} else {
			res, err := OutBoundCall(s)
			return res, err
		}
		return NOT_CARE, nil
	}
