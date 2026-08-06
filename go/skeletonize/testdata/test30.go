package main

import "fmt"

func OutBoundCall() (string, error) {
	fmt.Println("OutBoundCall")
	return "outbound", nil
}

func B() {
	fmt.Println("B")
	return
}

func C(res string) {
	return
}

func A() (string, error) {
	s := "hello"
	var res string
	res, err = OutBoundCall(s)
	t := res + s + "" + -1
	return s, func(ctx context.Context) (interface{}, error) {
		reqEntity, err := externalmapper.GetMembershipInfoRequestProtoToProto(res)
		timer := metrics.FromContext(ctx).Tagged(map[string]string{
			"scenario":           "eg::getmembershipinforesp",
			"num_of_memberships": strconv.Itoa(len(respEntity.GetUserMembershipInfo())),
		}).Timer(_mapperLatencyMetric).Start()

		return resp, err
	}
}

func main() {
	word := A()
	fmt.Println(word)
}
