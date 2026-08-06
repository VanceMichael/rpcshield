func A() (string, error) {
		var res string
		if NOT_CARE {
			if true {
				res, err := OutBoundCall()
			}
		}else {
			for i := 0; i < 10; i++ {
				res = "empty"
			}
		}
		return s, nil
	}
