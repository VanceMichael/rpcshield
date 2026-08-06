func A() (string, error) {
		var res string
		if NOT_CARE {
			res, err := OutBoundCall()
		} else if NOT_CARE {
			B(res)
		} else {
			res, err := OutBoundCall()
		}
		return s, nil
	}
