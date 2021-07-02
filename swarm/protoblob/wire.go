package protoblob

type (
	FetchBlobResponse struct {
		Header *FetchBlobResponseHeader `json:"header,omitempty"`
		Body   *FetchBlobResponseBody   `json:"body,omitempty"`
	}

	FetchBlobResponseHeader struct {
		Missing bool `json:"missing,omitempty"`
	}

	FetchBlobResponseBody struct {
		Data []byte `json:"data"`
		End  bool   `json:"end"`
	}
)
