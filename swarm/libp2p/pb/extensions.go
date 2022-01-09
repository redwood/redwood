package pb

import (
	"redwood.dev/blob"
	"redwood.dev/swarm/protohush"
	"redwood.dev/types"
)

func MakeBlobProtobuf_FetchManifest(blobID blob.ID) *BlobMessage {
	return &BlobMessage{
		Payload: &BlobMessage_FetchManifest_{
			FetchManifest: &BlobMessage_FetchManifest{Id: blobID.ToProtobuf()},
		},
	}
}

func MakeBlobProtobuf_SendManifest(manifest blob.Manifest, exists bool) *BlobMessage {
	return &BlobMessage{
		Payload: &BlobMessage_SendManifest_{
			SendManifest: &BlobMessage_SendManifest{Manifest: manifest.ToProtobuf(), Exists: exists},
		},
	}
}

func MakeBlobProtobuf_FetchChunk(sha3 types.Hash) *BlobMessage {
	return &BlobMessage{
		Payload: &BlobMessage_FetchChunk_{
			FetchChunk: &BlobMessage_FetchChunk{Sha3: sha3.Bytes()},
		},
	}
}

func MakeBlobProtobuf_SendChunk(chunk []byte, exists bool) *BlobMessage {
	return &BlobMessage{
		Payload: &BlobMessage_SendChunk_{
			SendChunk: &BlobMessage_SendChunk{Chunk: chunk, Exists: exists},
		},
	}
}

func MakeHushProtobuf_DHPubkeyAttestations(attestations []protohush.DHPubkeyAttestation) *HushMessage {
	return &HushMessage{
		Payload: &HushMessage_DhPubkeyAttestations{
			DhPubkeyAttestations: &HushMessage_DHPubkeyAttestations{Attestations: attestations},
		},
	}
}

func MakeHushProtobuf_ProposeIndividualSession(encryptedProposal []byte) *HushMessage {
	return &HushMessage{
		Payload: &HushMessage_ProposeIndividualSession_{
			ProposeIndividualSession: &HushMessage_ProposeIndividualSession{
				EncryptedProposal: encryptedProposal,
			},
		},
	}
}

func MakeHushProtobuf_RespondToIndividualSession(approval protohush.IndividualSessionResponse) *HushMessage {
	return &HushMessage{
		Payload: &HushMessage_RespondToIndividualSession_{
			RespondToIndividualSession: &HushMessage_RespondToIndividualSession{
				Approval: &approval,
			},
		},
	}
}

func MakeHushProtobuf_SendIndividualMessage(msg protohush.IndividualMessage) *HushMessage {
	return &HushMessage{
		Payload: &HushMessage_SendIndividualMessage_{
			SendIndividualMessage: &HushMessage_SendIndividualMessage{
				Message: &msg,
			},
		},
	}
}

func MakeHushProtobuf_SendGroupMessage(msg protohush.GroupMessage) *HushMessage {
	return &HushMessage{
		Payload: &HushMessage_SendGroupMessage_{
			SendGroupMessage: &HushMessage_SendGroupMessage{
				Message: &msg,
			},
		},
	}
}
