package dal

import (
	"github.com/vahdet/tafalk-user-store/models"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
	"github.com/fatih/structs"
	"errors"
	"fmt"
	"github.com/go-redis/redis"
)

const (
	idKey = "userid"
	userNameSetKey = "usernames"
	emailSetKey = "emails"
	userKeyPrefix = "user"
	keySeparator = ":"
)

type UserDal struct{}

func NewUserDal() *UserDal {
	return &UserDal{}
}

func (dal *UserDal) Get(id int64) (*models.User, error) {
	var user models.User
	// returns map[string]string
	var res, err = client.HGetAll(getPrefixedDataStoreId(id)).Result()
	if err != nil {
		log.WithFields(log.Fields{
			"id": id,
		}).Error(fmt.Sprintf("getting failed: '%#v'", err))
		return nil, err
	}
	// convert map[string]string to struct
	mapstructure.Decode(res, &user)
	if err != nil {
		log.WithFields(log.Fields{
			"id": id,
		}).Error(fmt.Sprintf("decoding gathered result failed: '%#v'", err))
		return nil, err
	}
	return &user, nil
}

func (dal *UserDal) Create(user *models.User) error {
	//Check if USERNAME and EMAIL already exists in one query
	pipe := client.TxPipeline()

	userNameScoreCmd := pipe.ZScore(userNameSetKey, user.Name)
	emailScoreCmd := pipe.ZScore(emailSetKey, user.Email)

	_, err := pipe.Exec()

	if err != nil {
		log.WithFields(log.Fields{
			"username": user.Name,
			"email": user.Email,
		}).Error(fmt.Sprintf("getting score of username and/or email failed: '%#v'", err))
		return err
	}

	userNameScore := userNameScoreCmd.Val()
	emailScore := emailScoreCmd.Val()

	if userNameScore > 0 || emailScore > 0 {
		log.WithFields(log.Fields{
			"username": user.Name,
			"email": user.Email,
		}).Error("The username and/or email already exists")
		return errors.New("user name and/or or email already exists")
	}

	// If everything is OK,...
	// Get Next Id
	incrResult, err := client.Incr(idKey).Result()
	if err != nil {
		log.Error(fmt.Sprintf("incrementing Id Key failed: '%#v'", err))
		return err
	}
	// Set the Id of the input of the autogenerated one
	user.Id = incrResult
	// ... add in EMAIL, USERNAME sorted sets and create USER itself
	pipeCreate := client.TxPipeline()
	pipeCreate.ZAddNX(userNameSetKey, redis.Z{Score: float64(incrResult), Member: user.Name})
	pipeCreate.ZAddNX(emailSetKey, redis.Z{Score: float64(incrResult), Member: user.Email})
	pipeCreate.HMSet(getPrefixedDataStoreId(incrResult), structs.Map(user))
	// Execute pipelined creation commands
	_, err = pipeCreate.Exec()

	if err != nil {
		log.WithFields(log.Fields{
			"username": user.Name,
			"email": user.Email,
		}).Error(fmt.Sprintf("creating user failed: '%#v'", err))
		return err
	}
	return nil
}

func (dal *UserDal) Update(id int64, user *models.User) error {
	//Check if USERNAME and EMAIL already exists in one query
	pipe := client.TxPipeline()

	userNameScoreCmd := pipe.ZScore(userNameSetKey, user.Name)
	emailScoreCmd := pipe.ZScore(emailSetKey, user.Email)

	_, err := pipe.Exec()

	if err != nil {
		log.WithFields(log.Fields{
			"username": user.Name,
			"email": user.Email,
		}).Error(fmt.Sprintf("getting score of username and/or email failed: '%#v'", err))
		return err
	}

	userNameScore := userNameScoreCmd.Val()
	emailScore := emailScoreCmd.Val()

	if userNameScore == 0 || emailScore == 0 {
		log.WithFields(log.Fields{
			"username": user.Name,
			"email": user.Email,
		}).Error("The username and/or email does not exists")
		return errors.New("user name and/or or email does not exists")
	}

	// If everything is OK,...
	// ... edit in EMAIL, USERNAME sorted sets and update USER itself

	pipeUpdate := client.TxPipeline()
	pipeUpdate.ZAddXX(userNameSetKey, redis.Z{Score: float64(user.Id), Member: user.Name})
	pipeUpdate.ZAddXX(emailSetKey, redis.Z{Score: float64(user.Id), Member: user.Email})
	pipeUpdate.HMSet(getPrefixedDataStoreId(user.Id), structs.Map(user))
	// Execute pipelined creation commands
	_, err = pipeUpdate.Exec()

	if err != nil {
		log.WithFields(log.Fields{
			"username": user.Name,
			"email": user.Email,
		}).Error(fmt.Sprintf("updating user failed: '%#v'", err))
		return err
	}
	return nil
}

func (dal *UserDal) Delete(id int64) error {
	var user models.User
	// returns map[string]string
	var res, err = client.HGetAll(getPrefixedDataStoreId(id)).Result()
	if err != nil {
		log.WithFields(log.Fields{
			"id": id,
		}).Error(fmt.Sprintf("getting failed: '%#v'", err))
		return err
	}
	// convert map[string]string to struct
	mapstructure.Decode(res, &user)
	if err != nil {
		log.WithFields(log.Fields{
			"id": id,
		}).Error(fmt.Sprintf("decoding gathered result failed: '%#v'", err))
		return err
	}

	// Remove in EMAIL, USERNAME sorted sets and delete USER itself
	pipeDelete := client.TxPipeline()
	pipeDelete.ZRem(userNameSetKey, user.Name)
	pipeDelete.ZRem(emailSetKey, user.Email)
	pipeDelete.Del(getPrefixedDataStoreId(user.Id))
	// Execute pipelined creation commands
	_, err = pipeDelete.Exec()

	if err != nil {
		log.WithFields(log.Fields{
			"username": user.Name,
			"email": user.Email,
		}).Error(fmt.Sprintf("deleting user failed: '%#v'", err))
		return err
	}
	return nil
}

func (dal *UserDal) Count(id int64) (int64, error) {
	return client.Exists(getPrefixedDataStoreId(id)).Result()
}

func getPrefixedDataStoreId(id int64) string {
//return userKeyPrefix + keySeparator + id
	return fmt.Sprintf("%s%s%d", userKeyPrefix, keySeparator, id)
}