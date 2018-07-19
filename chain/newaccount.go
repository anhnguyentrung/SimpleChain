package chain

import "blockchain/crypto"

type NewAccount struct {
	Creator AccountName
	Name 	AccountName
	Owner 	Authority
	Active 	Authority
}

func CreateNewAccount(a AccountName, creator AccountName) *Action {
	permissions := []PermissionLevel{
		{Actor:creator, Permission:"active"},
	}
	ownerPrivateKey, _ := crypto.NewPrivateKeyForAccount(a, OWNER)
	ownerPublicKey := ownerPrivateKey.PublicKey()
	owner := Authority{
		Threshold:	1,
		Keys:		[]KeyWeight{
			{Key:ownerPublicKey, Weight:1},
		},
		Accounts:	[]PermissionLevelWeight{},
	}
	activePrivateKey, _ := crypto.NewPrivateKeyForAccount(a, ACTIVE)
	activePublicKey := activePrivateKey.PublicKey()
	active := Authority{
		Threshold:	1,
		Keys:		[]KeyWeight{
			{Key:activePublicKey, Weight:1},
		},
		Accounts:	[]PermissionLevelWeight{},
	}
	newAccount := NewAccount{
		Creator:creator,
		Name:	a,
		Owner:	owner,
		Active:	active,

	}
	return &Action{
		creator,
		NEW_ACCOUNT,
		permissions,
		NewActionData(newAccount),
	}
}