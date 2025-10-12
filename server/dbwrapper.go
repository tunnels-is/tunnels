package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tunnels-is/tunnels/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	USERS_DATABASE   = "users"
	USERS_COLLECTION = "users"

	DEVICE_DATABASE   = "devices"
	DEVICE_COLLECTION = "devices"

	ORG_DATABASE   = "orgs"
	ORG_COLLECTION = "orgs"

	GROUP_DATABASE   = "groups"
	GROUP_COLLECTION = "groups"

	SERVER_DATABASE   = "servers"
	SERVER_COLLECTION = "servers"
)

var DB *mongo.Client

func ConnectToDB(connectionString string) (err error) {
	defer BasicRecover()

	var mongoMinSockets uint64 = 1500
	var mongoMaxSockets uint64 = 2000

	opt := options.Client()
	opt.SetMinPoolSize(mongoMinSockets)
	opt.SetMaxPoolSize(mongoMaxSockets)
	opt.SetHeartbeatInterval(20 * time.Second)
	opt.SetServerSelectionTimeout(5 * time.Second)
	opt.SetConnectTimeout(10 * time.Second)
	opt.SetTimeout(11 * time.Second)

	DB, err = mongo.Connect(context.Background(), opt.ApplyURI(connectionString))
	if err != nil {
		ERR(3, err)
		ADMIN(3, "Database error, unable to connect local")
		return
	}

	err = DB.Ping(context.Background(), nil)
	if err != nil {
		_ = DB.Disconnect(context.TODO())
		ERR(3, err)
		ADMIN(3, "Database error, unable to ping local")
		return
	}

	INFO(3, "DATABASE CONNECTED")
	return
}

func DB_DeleteDeviceByID(id primitive.ObjectID) (err error) {
	if BBOLTEnabled {
		return BBolt_DeleteDeviceByID(objectIDToString(id))
	}
	defer BasicRecover()

	opt := options.Delete()

	filter := bson.M{"_id": id}
	_, err = DB.Database(DEVICE_DATABASE).
		Collection(DEVICE_COLLECTION).
		DeleteOne(
			context.Background(),
			filter,
			opt,
		)
	if err != nil {
		ADMIN(3, "Unable to delete device by id: ", id, err)
		return err
	}

	return
}

func DB_UpdateDevice(D *types.Device) (err error) {
	if BBOLTEnabled {
		return BBolt_UpdateDevice(D)
	}
	defer BasicRecover()

	filter := bson.M{
		"_id": D.ID,
	}

	res, err := DB.Database(DEVICE_DATABASE).
		Collection(DEVICE_COLLECTION).
		UpdateOne(
			context.Background(),
			filter,
			bson.D{
				{
					Key: "$set",
					Value: bson.D{
						{Key: "Tag", Value: D.Tag},
					},
				},
			},
			options.Update(),
		)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil
		}
		ADMIN(3, "Unable to update device", D.ID.Hex(), err)
		return err
	}
	if res.MatchedCount == 0 {
		ADMIN(3, "Unable to update device(no match count)", D.ID.Hex(), err)
		return errors.New("unable to modify document")
	}

	return
}

func DB_GetDevices(limit, offset int64) (DL []*types.Device, err error) {
	if BBOLTEnabled {
		return BBolt_GetDevices(limit, offset)
	}
	defer BasicRecover()

	opt := options.Find()
	opt.SetLimit(limit)
	opt.SetSkip(offset)

	filter := bson.M{}

	cursor, err := DB.Database(DEVICE_DATABASE).
		Collection(DEVICE_COLLECTION).
		Find(
			context.Background(),
			filter,
			opt,
		)
	if err != nil {
		ADMIN(3, "Unable to find device: ", err)
		return nil, err
	}

	DL = make([]*types.Device, 0)
	for cursor.Next(context.TODO()) {
		D := new(types.Device)
		err = cursor.Decode(D)
		if err != nil {
			ADMIN(3, "Unable to decode user to struct: ", err)
			continue
		}
		DL = append(DL, D)
	}

	return
}

func DB_getUsers(limit, offset int64) (UL []*User, err error) {
	if BBOLTEnabled {
		return BBolt_getUsers(limit, offset)
	}
	defer BasicRecover()

	opt := options.Find()
	opt.SetLimit(limit)
	opt.SetSkip(offset)

	filter := bson.M{}

	cursor, err := DB.Database(USERS_DATABASE).
		Collection(USERS_COLLECTION).
		Find(
			context.Background(),
			filter,
			opt,
		)
	if err != nil {
		ADMIN(3, "Unable to find users: ", err)
		return nil, err
	}

	UL = make([]*User, 0)
	for cursor.Next(context.TODO()) {
		D := new(User)
		err = cursor.Decode(D)
		if err != nil {
			ADMIN(3, "Unable to decode user to struct: ", err)
			continue
		}
		UL = append(UL, D)
	}

	return
}

func DB_findUserByAPIKey(Key string) (USER *User, err error) {
	if BBOLTEnabled {
		return BBolt_findUserByAPIKey(Key)
	}
	defer BasicRecover()

	USER = new(User)

	err = DB.Database(USERS_DATABASE).
		Collection(USERS_COLLECTION).
		FindOne(
			context.Background(),
			bson.M{"APIKey": Key},
			options.FindOne(),
		).
		Decode(&USER)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		ADMIN(3, err)
	}

	return
}

func DB_findUserByID(UID primitive.ObjectID) (USER *User, err error) {
	if BBOLTEnabled {
		return BBolt_findUserByID(objectIDToString(UID))
	}
	defer BasicRecover()

	USER = new(User)

	err = DB.Database(USERS_DATABASE).
		Collection(USERS_COLLECTION).
		FindOne(
			context.Background(),
			bson.M{"_id": UID},
			options.FindOne(),
		).
		Decode(&USER)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		ADMIN(3, err)
	}

	return
}

func DB_CreateUser(U *User) (err error) {
	if BBOLTEnabled {
		return BBolt_CreateUser(U)
	}
	defer BasicRecover()
	_, err = DB.Database(USERS_DATABASE).
		Collection(USERS_COLLECTION).
		InsertOne(
			context.Background(),
			U,
			options.InsertOne(),
		)
	if err != nil {
		ADMIN(3, err)
	}

	return
}

func DB_findUserByEmail(Email string) (USER *User, err error) {
	if BBOLTEnabled {
		return BBolt_findUserByEmail(Email)
	}
	USER = new(User)
	err = DB.Database(USERS_DATABASE).
		Collection(USERS_COLLECTION).
		FindOne(
			context.Background(),
			bson.M{"Email": Email},
			options.FindOne(),
		).
		Decode(&USER)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		ADMIN(3, err)
	}

	return
}

func DB_updateUserDeviceTokens(TU *UPDATE_USER_TOKENS) (err error) {
	if BBOLTEnabled {
		return BBolt_updateUserDeviceTokens(TU)
	}
	defer BasicRecover()

	_, err = DB.Database(USERS_DATABASE).
		Collection(USERS_COLLECTION).
		UpdateOne(
			context.Background(),
			bson.M{"_id": TU.ID},
			bson.D{
				primitive.E{
					Key: "$set",
					Value: bson.D{
						bson.E{Key: "Tokens", Value: TU.Tokens},
						bson.E{Key: "Version", Value: TU.Version},
					},
				},
			},
		)
	if err != nil {
		ADMIN(3, err)
	}

	return
}

func DB_updateUserSubTime(u *User) (err error) {
	if BBOLTEnabled {
		return BBolt_updateUserSubTime(u)
	}
	defer BasicRecover()

	filter := bson.M{
		"Email": u.Email,
	}

	res, err := DB.Database(USERS_DATABASE).
		Collection(USERS_COLLECTION).
		UpdateOne(
			context.Background(),
			filter,
			bson.D{
				{
					Key: "$set",
					Value: bson.D{
						{Key: "SubExpiration", Value: u.SubExpiration},
					},
				},
			},
			options.Update(),
		)
	if err != nil {
		ADMIN(3, "Could not update user sub time: ", err)
		return err
	}

	if res.MatchedCount == 0 {
		ADMIN(3, "Could not update user sub time: ", err)
		return errors.New("unable to modify document")
	}

	return
}

func DB_updateUser(UF *USER_UPDATE_FORM) (err error) {
	if BBOLTEnabled {
		return BBolt_updateUser(UF)
	}
	defer BasicRecover()

	filter := bson.M{
		"_id": UF.UID,
	}

	res, err := DB.Database(USERS_DATABASE).
		Collection(USERS_COLLECTION).
		UpdateOne(
			context.Background(),
			filter,
			bson.D{
				{
					Key: "$set",
					Value: bson.D{
						{Key: "APIKey", Value: UF.APIKey},
						{Key: "AdditionalInformation", Value: UF.AdditionalInformation},
					},
				},
			},
			options.Update(),
		)
	if err != nil {
		ADMIN(3, "Could not update user: ", err)
		return err
	}

	if res.MatchedCount == 0 {
		ADMIN(3, "Could not update user: ", err)
		return errors.New("unable to modify document")
	}

	return
}

func DB_updateUserAdmin(UF *USER_ADMIN_UPDATE_FORM) (err error) {
	if BBOLTEnabled {
		return BBolt_updateUserAdmin(UF)
	}
	defer BasicRecover()

	filter := bson.M{
		"_id": UF.TargetUserID,
	}

	updateFields := bson.D{}

	if UF.Email != "" {
		updateFields = append(updateFields, bson.E{Key: "Email", Value: UF.Email})
	}

	updateFields = append(updateFields, bson.E{Key: "Disabled", Value: UF.Disabled})
	updateFields = append(updateFields, bson.E{Key: "IsManager", Value: UF.IsManager})
	updateFields = append(updateFields, bson.E{Key: "Trial", Value: UF.Trial})

	res, err := DB.Database(USERS_DATABASE).
		Collection(USERS_COLLECTION).
		UpdateOne(
			context.Background(),
			filter,
			bson.D{
				{
					Key:   "$set",
					Value: updateFields,
				},
			},
			options.Update(),
		)
	if err != nil {
		ADMIN(3, "Could not admin update user: ", err)
		return err
	}

	if res.MatchedCount == 0 {
		ADMIN(3, "Could not admin update user: user not found")
		return errors.New("unable to modify document")
	}

	return
}

func DB_toggleUserSubscriptionStatus(UF *USER_UPDATE_SUB_FORM) (err error) {
	if BBOLTEnabled {
		return BBolt_toggleUserSubscriptionStatus(UF)
	}
	defer BasicRecover()

	filter := bson.M{
		"Email": UF.Email,
	}

	res, err := DB.Database(USERS_DATABASE).
		Collection(USERS_COLLECTION).
		UpdateOne(
			context.Background(),
			filter,
			bson.D{
				{
					Key: "$set",
					Value: bson.D{
						{Key: "CancelSub", Value: UF.Disable},
					},
				},
			},
			options.Update(),
		)
	if err != nil {
		ADMIN(3, "Could not update user sub status: ", err)
		return err
	}

	if res.MatchedCount == 0 {
		ADMIN(3, "Could not update user sub status: ", err)
		return errors.New("unable to modify document")
	}

	return
}

func DB_userUpdateTwoFactorCodes(TFP *TWO_FACTOR_DB_PACKAGE) (err error) {
	if BBOLTEnabled {
		return BBolt_userUpdateTwoFactorCodes(TFP)
	}
	defer BasicRecover()

	_, err = DB.Database(USERS_DATABASE).
		Collection(USERS_COLLECTION).
		UpdateOne(
			context.Background(),
			bson.M{"_id": TFP.UID},
			bson.D{
				{
					Key: "$set",
					Value: bson.D{
						{Key: "TwoFactorCode", Value: TFP.Code},
						{Key: "RecoveryCodes", Value: TFP.Recovery},
						{Key: "TwoFactorEnabled", Value: true},
					},
				},
			},
		)
	if err != nil {
		ADMIN(3, err)
	}

	return
}

func DB_userResetPassword(user *User) error {
	if BBOLTEnabled {
		return BBolt_userResetPassword(user)
	}
	defer BasicRecover()

	res, err := DB.Database(USERS_DATABASE).
		Collection(USERS_COLLECTION).
		UpdateOne(
			context.Background(),
			bson.M{"_id": user.ID},
			bson.D{
				{
					Key: "$set",
					Value: bson.D{
						{Key: "Password", Value: user.Password},
						{Key: "Tokens", Value: make([]*DeviceToken, 0)},
						{Key: "ResetCode", Value: ""},
					},
				},
			},
		)
	if err != nil {
		ADMIN(3, "Unable to modify user password: ", user.ID)
		return err
	}

	if res.MatchedCount == 0 {
		ADMIN(3, "Unable to modify user password: ", user.ID)
		return errors.New("user password could not be modified")
	}

	return nil
}

func DB_FindServersWithoutGroups(limit, offset int64) (DL []*types.Server, err error) {
	if BBOLTEnabled {
		return BBolt_FindServersWithoutGroups(limit, offset)
	}
	defer BasicRecover()

	opt := options.Find()
	opt.SetLimit(limit)
	opt.SetSkip(offset)

	filter := bson.D{
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "Groups", Value: bson.D{{Key: "$exists", Value: false}}}},
			bson.D{{Key: "Groups", Value: nil}},
			bson.D{{Key: "Groups", Value: bson.D{{Key: "$size", Value: 0}}}},
		}},
	}

	cursor, err := DB.Database(SERVER_DATABASE).
		Collection(SERVER_COLLECTION).
		Find(
			context.Background(),
			filter,
			opt,
		)
	if err != nil {
		ADMIN(3, "Unable to find online devices: ", err)
		return nil, err
	}

	DL = make([]*types.Server, 0)
	for cursor.Next(context.TODO()) {
		D := new(types.Server)
		err = cursor.Decode(D)
		if err != nil {
			ADMIN(3, "Unable to decode device to struct: ", err)
			continue
		}
		DL = append(DL, D)
	}

	return
}

func DB_FindServersByGroups(groups []primitive.ObjectID, limit, offset int64) (DL []*types.Server, err error) {
	if BBOLTEnabled {
		return BBolt_FindServersByGroups(objectIDSliceToString(groups), limit, offset)
	}
	defer BasicRecover()

	opt := options.Find()
	opt.SetLimit(limit)
	opt.SetSkip(offset)

	filter := bson.D{
		{Key: "Groups", Value: bson.D{
			{Key: "$in", Value: groups},
		}},
	}

	cursor, err := DB.Database(SERVER_DATABASE).
		Collection(SERVER_COLLECTION).
		Find(
			context.Background(),
			filter,
			opt,
		)
	if err != nil {
		ADMIN(3, "Unable to find online devices: ", err)
		return nil, err
	}

	DL = make([]*types.Server, 0)
	for cursor.Next(context.TODO()) {
		D := new(types.Server)
		err = cursor.Decode(D)
		if err != nil {
			ADMIN(3, "Unable to decode device to struct: ", err)
			continue
		}
		DL = append(DL, D)
	}

	return
}

func DB_FindEntitiesByGroupID(id primitive.ObjectID, objType string, limit, offset int64) (IL []any, err error) {
	if BBOLTEnabled {
		return BBolt_FindEntitiesByGroupID(objectIDToString(id), objType, limit, offset)
	}
	defer BasicRecover()

	opt := options.Find()
	opt.SetLimit(limit)
	opt.SetSkip(offset)

	database := ""
	collection := ""
	switch objType {
	case "user":
		database = USERS_DATABASE
		collection = USERS_COLLECTION
	case "server":
		database = SERVER_DATABASE
		collection = SERVER_COLLECTION
	case "device":
		database = DEVICE_DATABASE
		collection = DEVICE_COLLECTION
	default:
		return nil, fmt.Errorf("unknown type")
	}

	filter := bson.D{
		{Key: "Groups", Value: id},
	}

	cursor, err := DB.Database(database).
		Collection(collection).
		Find(
			context.Background(),
			filter,
			opt,
		)
	if err != nil {
		ADMIN(3, "Unable to find online devices: ", err)
		return nil, err
	}

	IL = make([]any, 0)
	for cursor.Next(context.TODO()) {
		switch objType {
		case "server":
			E := new(types.Server)
			err = cursor.Decode(E)
			if err != nil {
				ADMIN(3, "Unable to decode device to struct: ", err)
				continue
			}
			IL = append(IL, E)
		case "user":
			E := new(User)
			err = cursor.Decode(E)
			if err != nil {
				ADMIN(3, "Unable to decode device to struct: ", err)
				continue
			}
			IL = append(IL, E)
		case "device":
			E := new(types.Device)
			err = cursor.Decode(E)
			if err != nil {
				ADMIN(3, "Unable to decode device to struct: ", err)
				continue
			}
			IL = append(IL, E)
		}
	}

	return
}

func DB_UpdateGroup(G *Group) (err error) {
	if BBOLTEnabled {
		return BBolt_UpdateGroup(G)
	}
	defer BasicRecover()

	filter := bson.M{
		"_id": G.ID,
	}

	res, err := DB.Database(GROUP_DATABASE).
		Collection(GROUP_COLLECTION).
		UpdateOne(
			context.Background(),
			filter,
			bson.D{
				{
					Key: "$set",
					Value: bson.D{
						{Key: "Tag", Value: G.Tag},
						{Key: "Description", Value: G.Description},
					},
				},
			},
			options.Update(),
		)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil
		}
		ADMIN(3, "Unable to update group", G.ID.Hex(), err)
		return err
	}
	if res.MatchedCount == 0 {
		ADMIN(3, "Unable to update group(no match count)", G.ID.Hex(), err)
		return errors.New("unable to modify document")
	}

	return
}

func DB_UpdateServer(S *types.Server) (RS *types.Server, err error) {
	if BBOLTEnabled {
		return BBolt_UpdateServer(S)
	}
	defer BasicRecover()

	filter := bson.M{
		"_id": S.ID,
	}

	err = DB.Database(SERVER_DATABASE).
		Collection(SERVER_COLLECTION).
		FindOneAndUpdate(
			context.Background(),
			filter,
			bson.D{
				{
					Key: "$set",
					Value: bson.D{
						{Key: "Tag", Value: S.Tag},
						{Key: "Country", Value: S.Country},
						{Key: "IP", Value: S.IP},
						{Key: "Port", Value: S.Port},
						{Key: "DataPort", Value: S.DataPort},
						{Key: "PubKey", Value: S.PubKey},
					},
				},
			},
			options.FindOneAndUpdate(),
		).Decode(&RS)
	if err != nil {
		ADMIN(3, "Unable to update server: ", S.ID.Hex())
		return nil, err
	}

	return
}

func DB_CreateDevice(D *types.Device) (err error) {
	if BBOLTEnabled {
		return BBolt_CreateDevice(D)
	}
	defer BasicRecover()

	_, err = DB.Database(DEVICE_DATABASE).
		Collection(DEVICE_COLLECTION).
		InsertOne(
			context.Background(),
			D,
			options.InsertOne(),
		)
	if err != nil {
		ADMIN(3, "Unable to create device: ", err)
		return err
	}

	return
}

func DB_CreateGroup(G *Group) (err error) {
	if BBOLTEnabled {
		return BBolt_CreateGroup(G)
	}
	defer BasicRecover()

	_, err = DB.Database(GROUP_DATABASE).
		Collection(GROUP_COLLECTION).
		InsertOne(
			context.Background(),
			G,
			options.InsertOne(),
		)
	if err != nil {
		ADMIN(3, "Unable to create group: ", err)
		return err
	}

	return
}

func DB_CreateServer(S *types.Server) (err error) {
	if BBOLTEnabled {
		return BBolt_CreateServer(S)
	}
	defer BasicRecover()

	_, err = DB.Database(SERVER_DATABASE).
		Collection(SERVER_COLLECTION).
		InsertOne(
			context.Background(),
			S,
			options.InsertOne(),
		)
	if err != nil {
		ADMIN(3, "Unable to create server: ", err)
		return err
	}

	return
}

func DB_FindServerByID(ID primitive.ObjectID) (S *types.Server, err error) {
	if BBOLTEnabled {
		return BBolt_FindServerByID(objectIDToString(ID))
	}
	defer BasicRecover()

	filter := bson.M{
		"_id": ID,
	}

	S = new(types.Server)
	err = DB.Database(SERVER_DATABASE).
		Collection(SERVER_COLLECTION).
		FindOne(
			context.Background(),
			filter,
			options.FindOne(),
		).Decode(S)
	if err != nil {
		ADMIN(3, "Could not find server by id: ", ID, " / ", err)
		return nil, err
	}

	return
}

func DB_WipeUserConfirmCode(UF *USER_ENABLE_QUERY) (err error) {
	if BBOLTEnabled {
		return BBolt_WipeUserConfirmCode(UF)
	}
	defer BasicRecover()

	filter := bson.M{
		"Email": UF.Email,
	}

	res, err := DB.Database(USERS_DATABASE).
		Collection(USERS_COLLECTION).
		UpdateOne(
			context.Background(),
			filter,
			bson.D{
				{
					Key: "$set",
					Value: bson.D{
						{Key: "ConfirmCode", Value: ""},
					},
				},
			},
			options.Update(),
		)
	if err != nil {
		ADMIN(3, "Could not enable user: ", err)
		return err
	}

	// INFO(3, "COUNTS:", res.MatchedCount, res.ModifiedCount)
	if res.MatchedCount == 0 {
		ADMIN(3, "Could not enable user, user no found: ", err)
		return errors.New("unable to modify document")
	}

	return
}

func DB_UserActivateKey(SubExpiration time.Time, Key *LicenseKey, userID primitive.ObjectID) (err error) {
	if BBOLTEnabled {
		return BBolt_UserActivateKey(SubExpiration, Key, objectIDToString(userID))
	}
	defer BasicRecover()

	filter := bson.M{
		"_id": userID,
	}

	res, err := DB.Database(USERS_DATABASE).
		Collection(USERS_COLLECTION).
		UpdateOne(
			context.Background(),
			filter,
			bson.D{
				{
					Key: "$set",
					Value: bson.D{
						{Key: "Disabled", Value: false},
						{Key: "Trial", Value: false},
						{Key: "SubExpiration", Value: SubExpiration},
						{Key: "Key", Value: Key},
					},
				},
			},
			options.Update(),
		)
	if err != nil {
		ADMIN(3, "Unable to update user post payment: ", userID, " / ", err)
		return err
	}

	if res.MatchedCount == 0 {
		ADMIN(3, "Unable to update user post payment: ", userID)
		return errors.New("unable to modify document")
	}

	return
}

func DB_AddToGroup(groupID primitive.ObjectID, typeID primitive.ObjectID, objType string) (err error) {
	if BBOLTEnabled {
		return BBolt_AddToGroup(objectIDToString(groupID), objectIDToString(typeID), objType)
	}
	defer BasicRecover()

	filter := bson.M{
		"_id": typeID,
	}

	database := ""
	collection := ""
	switch objType {
	case "user":
		database = USERS_DATABASE
		collection = USERS_COLLECTION
	case "server":
		database = SERVER_DATABASE
		collection = SERVER_COLLECTION
	case "device":
		database = DEVICE_DATABASE
		collection = DEVICE_COLLECTION
	default:
		return fmt.Errorf("unknown type")
	}

	res, err := DB.Database(database).
		Collection(collection).
		UpdateOne(
			context.Background(),
			filter,
			bson.D{
				{
					Key: "$push",
					Value: bson.D{
						{Key: "Groups", Value: groupID},
					},
				},
			},
			options.Update(),
		)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil
		}
		ADMIN(3, "Unable to update object", objType, typeID.Hex(), err)
		return err
	}
	if res.MatchedCount == 0 {
		ADMIN(3, "Unable to update (no match count)", objType, typeID.Hex(), err)
		return errors.New("unable to modify document")
	}

	return
}

func DB_RemoveFromGroup(groupID primitive.ObjectID, typeID primitive.ObjectID, objType string) (err error) {
	if BBOLTEnabled {
		return BBolt_RemoveFromGroup(objectIDToString(groupID), objectIDToString(typeID), objType)
	}
	defer BasicRecover()

	filter := bson.M{
		"_id": typeID,
	}

	database := ""
	collection := ""
	switch objType {
	case "user":
		database = USERS_DATABASE
		collection = USERS_COLLECTION
	case "server":
		database = SERVER_DATABASE
		collection = SERVER_COLLECTION
	case "device":
		database = DEVICE_DATABASE
		collection = DEVICE_COLLECTION
	default:
		return fmt.Errorf("unknown type")
	}

	res, err := DB.Database(database).
		Collection(collection).
		UpdateOne(
			context.Background(),
			filter,
			bson.D{
				{
					Key: "$pull",
					Value: bson.D{
						{Key: "Groups", Value: groupID},
					},
				},
			},
			options.Update(),
		)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil
		}
		ADMIN(3, "Unable to update object", objType, typeID.Hex(), err)
		return err
	}
	if res.MatchedCount == 0 {
		ADMIN(3, "Unable to update (no match count)", objType, typeID.Hex(), err)
		return errors.New("unable to modify document")
	}

	return
}

func DB_FindDeviceByID(id primitive.ObjectID) (dev *types.Device, err error) {
	if BBOLTEnabled {
		return BBolt_FindDeviceByID(objectIDToString(id))
	}
	defer BasicRecover()

	opt := options.FindOne()
	dev = new(types.Device)

	filter := bson.M{"_id": id}
	err = DB.Database(DEVICE_DATABASE).
		Collection(DEVICE_COLLECTION).
		FindOne(
			context.Background(),
			filter,
			opt,
		).Decode(&dev)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		ADMIN(3, "Unable to find device by id :", id, err)
		return nil, err
	}

	return
}

func DB_findGroupByID(id primitive.ObjectID) (G *Group, err error) {
	if BBOLTEnabled {
		return BBolt_findGroupByID(objectIDToString(id))
	}
	defer BasicRecover()

	opt := options.FindOne()
	G = new(Group)

	filter := bson.M{"_id": id}
	err = DB.Database(GROUP_DATABASE).
		Collection(GROUP_COLLECTION).
		FindOne(
			context.Background(),
			filter,
			opt,
		).Decode(&G)
	if err != nil {
		ADMIN(3, "Unable to find group by id: ", id, err)
		return nil, err
	}

	return
}

func DB_DeleteGroupByID(id primitive.ObjectID) (err error) {
	if BBOLTEnabled {
		return BBolt_DeleteGroupByID(objectIDToString(id))
	}
	defer BasicRecover()

	opt := options.Delete()

	filter := bson.M{"_id": id}
	_, err = DB.Database(GROUP_DATABASE).
		Collection(GROUP_COLLECTION).
		DeleteOne(
			context.Background(),
			filter,
			opt,
		)
	if err != nil {
		ADMIN(3, "Unable to delete group by id: ", id, err)
		return err
	}

	return
}

func DB_findGroups() (gl []*Group, err error) {
	if BBOLTEnabled {
		return BBolt_findGroups()
	}
	defer BasicRecover()

	opt := options.Find()

	filter := bson.M{}
	cursor, err := DB.Database(GROUP_DATABASE).
		Collection(GROUP_COLLECTION).Find(
		context.Background(),
		filter,
		opt,
	)
	if err != nil {
		ADMIN(3, "Unable to find groups: ", err)
		return nil, err
	}

	gl = make([]*Group, 0)
	for cursor.Next(context.TODO()) {
		D := new(Group)
		err = cursor.Decode(D)
		if err != nil {
			ADMIN(3, "Unable to decode group to struct: ", err)
			continue
		}
		gl = append(gl, D)
	}

	return
}
