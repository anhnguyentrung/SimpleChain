package chain

import "blockchain/crypto"

type NewAccount struct {
	Creator AccountName
	Name 	AccountName
	Owner 	Authority
	Active 	Authority
}

func CreateNewAccount(a AccountName, creator AccountName, publicKey crypto.PublicKey) *Action {
	permissions := []PermissionLevel{
		{Actor:creator, Permission:"active"},
	}
	owner := Authority{
		Threshold:	1,
		Keys:		[]KeyWeight{
			{Key:publicKey, Weight:1},
		},
		Accounts:	[]PermissionLevelWeight{},
	}
	active := Authority{
		Threshold:	1,
		Keys:		[]KeyWeight{
			{Key:publicKey, Weight:1},
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