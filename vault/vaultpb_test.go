// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: vault.proto

package vault

import (
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	github_com_gogo_protobuf_proto "github.com/gogo/protobuf/proto"
	proto "github.com/gogo/protobuf/proto"
	math "math"
	math_rand "math/rand"
	testing "testing"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

func BenchmarkAuthorizeMsgProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*AuthorizeMsg, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedAuthorizeMsg(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(dAtA)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAuthorizeMsgProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedAuthorizeMsg(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = dAtA
	}
	msg := &AuthorizeMsg{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAuthorizeMsg_ChallengeProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*AuthorizeMsg_Challenge, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedAuthorizeMsg_Challenge(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(dAtA)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAuthorizeMsg_ChallengeProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedAuthorizeMsg_Challenge(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = dAtA
	}
	msg := &AuthorizeMsg_Challenge{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAuthorizeMsg_SignedChallengeProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*AuthorizeMsg_SignedChallenge, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedAuthorizeMsg_SignedChallenge(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(dAtA)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAuthorizeMsg_SignedChallengeProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedAuthorizeMsg_SignedChallenge(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = dAtA
	}
	msg := &AuthorizeMsg_SignedChallenge{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAuthorizeMsg_ResponseProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*AuthorizeMsg_Response, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedAuthorizeMsg_Response(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(dAtA)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAuthorizeMsg_ResponseProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedAuthorizeMsg_Response(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = dAtA
	}
	msg := &AuthorizeMsg_Response{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkItemsReqProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*ItemsReq, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedItemsReq(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(dAtA)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkItemsReqProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedItemsReq(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = dAtA
	}
	msg := &ItemsReq{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkItemsRespProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*ItemsResp, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedItemsResp(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(dAtA)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkItemsRespProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedItemsResp(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = dAtA
	}
	msg := &ItemsResp{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkFetchReqProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*FetchReq, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedFetchReq(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(dAtA)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkFetchReqProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedFetchReq(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = dAtA
	}
	msg := &FetchReq{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkFetchRespProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*FetchResp, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedFetchResp(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(dAtA)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkFetchRespProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedFetchResp(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = dAtA
	}
	msg := &FetchResp{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStoreReqProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StoreReq, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedStoreReq(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(dAtA)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStoreReqProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedStoreReq(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = dAtA
	}
	msg := &StoreReq{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStoreReq_HeaderProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StoreReq_Header, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedStoreReq_Header(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(dAtA)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStoreReq_HeaderProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedStoreReq_Header(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = dAtA
	}
	msg := &StoreReq_Header{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStoreReq_PayloadProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StoreReq_Payload, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedStoreReq_Payload(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(dAtA)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStoreReq_PayloadProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedStoreReq_Payload(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = dAtA
	}
	msg := &StoreReq_Payload{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStoreRespProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StoreResp, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedStoreResp(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(dAtA)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStoreRespProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedStoreResp(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = dAtA
	}
	msg := &StoreResp{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkDeleteReqProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*DeleteReq, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedDeleteReq(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(dAtA)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkDeleteReqProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedDeleteReq(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = dAtA
	}
	msg := &DeleteReq{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkDeleteRespProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*DeleteResp, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedDeleteResp(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(dAtA)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkDeleteRespProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedDeleteResp(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = dAtA
	}
	msg := &DeleteResp{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkSetUserCapabilitiesReqProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*SetUserCapabilitiesReq, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedSetUserCapabilitiesReq(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(dAtA)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkSetUserCapabilitiesReqProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedSetUserCapabilitiesReq(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = dAtA
	}
	msg := &SetUserCapabilitiesReq{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkSetUserCapabilitiesRespProtoMarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*SetUserCapabilitiesResp, 10000)
	for i := 0; i < 10000; i++ {
		pops[i] = NewPopulatedSetUserCapabilitiesResp(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(pops[i%10000])
		if err != nil {
			panic(err)
		}
		total += len(dAtA)
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkSetUserCapabilitiesRespProtoUnmarshal(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	datas := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		dAtA, err := github_com_gogo_protobuf_proto.Marshal(NewPopulatedSetUserCapabilitiesResp(popr, false))
		if err != nil {
			panic(err)
		}
		datas[i] = dAtA
	}
	msg := &SetUserCapabilitiesResp{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += len(datas[i%10000])
		if err := github_com_gogo_protobuf_proto.Unmarshal(datas[i%10000], msg); err != nil {
			panic(err)
		}
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAuthorizeMsgSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*AuthorizeMsg, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedAuthorizeMsg(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAuthorizeMsg_ChallengeSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*AuthorizeMsg_Challenge, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedAuthorizeMsg_Challenge(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAuthorizeMsg_SignedChallengeSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*AuthorizeMsg_SignedChallenge, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedAuthorizeMsg_SignedChallenge(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkAuthorizeMsg_ResponseSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*AuthorizeMsg_Response, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedAuthorizeMsg_Response(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkItemsReqSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*ItemsReq, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedItemsReq(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkItemsRespSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*ItemsResp, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedItemsResp(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkFetchReqSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*FetchReq, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedFetchReq(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkFetchRespSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*FetchResp, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedFetchResp(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStoreReqSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StoreReq, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedStoreReq(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStoreReq_HeaderSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StoreReq_Header, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedStoreReq_Header(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStoreReq_PayloadSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StoreReq_Payload, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedStoreReq_Payload(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkStoreRespSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*StoreResp, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedStoreResp(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkDeleteReqSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*DeleteReq, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedDeleteReq(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkDeleteRespSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*DeleteResp, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedDeleteResp(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkSetUserCapabilitiesReqSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*SetUserCapabilitiesReq, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedSetUserCapabilitiesReq(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

func BenchmarkSetUserCapabilitiesRespSize(b *testing.B) {
	popr := math_rand.New(math_rand.NewSource(616))
	total := 0
	pops := make([]*SetUserCapabilitiesResp, 1000)
	for i := 0; i < 1000; i++ {
		pops[i] = NewPopulatedSetUserCapabilitiesResp(popr, false)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total += pops[i%1000].Size()
	}
	b.SetBytes(int64(total / b.N))
}

//These tests are generated by github.com/gogo/protobuf/plugin/testgen
