package pb

import (
	"redwood.dev/blob"
	"redwood.dev/crypto"
	"redwood.dev/state"
	"redwood.dev/swarm"
	"redwood.dev/swarm/protohush"
	"redwood.dev/swarm/prototree"
	"redwood.dev/tree"
	"redwood.dev/types"
)

func MakeAuthProtobuf_ChallengeRequest() *AuthMessage {
	return &AuthMessage{
		Payload: &AuthMessage_ChallengeRequest_{
			ChallengeRequest: &AuthMessage_ChallengeRequest{},
		},
	}
}

func MakeAuthProtobuf_Challenge(challenge crypto.ChallengeMsg) *AuthMessage {
	return &AuthMessage{
		Payload: &AuthMessage_Challenge_{
			Challenge: &AuthMessage_Challenge{Challenge: challenge},
		},
	}
}

func MakeAuthProtobuf_Signatures(challenges []crypto.ChallengeMsg, signatures []crypto.Signature, asymEncPubkeys []crypto.AsymEncPubkey) *AuthMessage {
	return &AuthMessage{
		Payload: &AuthMessage_Signatures_{
			Signatures: &AuthMessage_Signatures{Challenges: challenges, Signatures: signatures, AsymEncPubkeys: asymEncPubkeys},
		},
	}
}

func MakeAuthProtobuf_Ucan(ucan string) *AuthMessage {
	return &AuthMessage{
		Payload: &AuthMessage_Ucan_{
			Ucan: &AuthMessage_Ucan{Ucan: ucan},
		},
	}
}

func MakeBlobProtobuf_FetchManifest(blobID blob.ID) *BlobMessage {
	return &BlobMessage{
		Payload: &BlobMessage_FetchManifest_{
			FetchManifest: &BlobMessage_FetchManifest{BlobID: blobID},
		},
	}
}

func MakeBlobProtobuf_SendManifest(manifest blob.Manifest, exists bool) *BlobMessage {
	return &BlobMessage{
		Payload: &BlobMessage_SendManifest_{
			SendManifest: &BlobMessage_SendManifest{Manifest: manifest, Exists: exists},
		},
	}
}

func MakeBlobProtobuf_FetchChunk(sha3 types.Hash) *BlobMessage {
	return &BlobMessage{
		Payload: &BlobMessage_FetchChunk_{
			FetchChunk: &BlobMessage_FetchChunk{SHA3: sha3.Bytes()},
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

func MakeHushProtobuf_PubkeyBundles(bundles []protohush.PubkeyBundle) *HushMessage {
	return &HushMessage{
		Payload: &HushMessage_PubkeyBundles_{
			PubkeyBundles: &HushMessage_PubkeyBundles{Bundles: bundles},
		},
	}
}

func MakeTreeProtobuf_Subscribe(stateURI string) *TreeMessage {
	return &TreeMessage{
		Payload: &TreeMessage_Subscribe_{
			Subscribe: &TreeMessage_Subscribe{StateURI: stateURI},
		},
	}
}

func MakeTreeProtobuf_Tx(tx tree.Tx) *TreeMessage {
	return &TreeMessage{
		Payload: &TreeMessage_Tx{
			Tx: &tx,
		},
	}
}

func MakeTreeProtobuf_EncryptedTx(tx prototree.EncryptedTx) *TreeMessage {
	return &TreeMessage{
		Payload: &TreeMessage_EncryptedTx_{
			EncryptedTx: &TreeMessage_EncryptedTx{EncryptedTx: tx},
		},
	}
}

func MakeTreeProtobuf_Ack(stateURI string, txID state.Version) *TreeMessage {
	return &TreeMessage{
		Payload: &TreeMessage_Ack_{
			Ack: &TreeMessage_Ack{StateURI: stateURI, TxID: txID},
		},
	}
}

func MakeTreeProtobuf_AnnounceP2PStateURI(stateURI string) *TreeMessage {
	return &TreeMessage{
		Payload: &TreeMessage_AnnounceP2PStateURI_{
			AnnounceP2PStateURI: &TreeMessage_AnnounceP2PStateURI{StateURI: stateURI},
		},
	}
}

func MakePeerProtobuf_AnnouncePeers(dialInfos []swarm.PeerDialInfo) *PeerMessage {
	return &PeerMessage{
		Payload: &PeerMessage_AnnouncePeers_{
			AnnouncePeers: &PeerMessage_AnnouncePeers{DialInfos: dialInfos},
		},
	}
}

// func MakeHushProtobuf_SendIndividualMessage(msg protohush.IndividualMessage) *HushMessage {
// 	return &HushMessage{
// 		Payload: &HushMessage_SendIndividualMessage_{
// 			SendIndividualMessage: &HushMessage_SendIndividualMessage{
// 				Message: &msg,
// 			},
// 		},
// 	}
// }

// func MakeHushProtobuf_SendGroupMessage(msg protohush.GroupMessage) *HushMessage {
// 	return &HushMessage{
// 		Payload: &HushMessage_SendGroupMessage_{
// 			SendGroupMessage: &HushMessage_SendGroupMessage{
// 				Message: &msg,
// 			},
// 		},
// 	}
// }
