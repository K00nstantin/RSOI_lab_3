package queue

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type RetryRequest struct {
	ID         string
	Method     string
	URL        string
	Headers    map[string]string
	Body       []byte
	RetryAt    time.Time
	RetryCount int
	MaxRetries int
}

type Queue struct {
	client *redis.Client
	ctx    context.Context
	key    string // Redis key для sorted set
}

const (
	defaultQueueKey = "retry_queue"
)

func NewQueue(redisClient *redis.Client) *Queue {
	if redisClient == nil {
		panic("redis client cannot be nil")
	}
	return &Queue{
		client: redisClient,
		ctx:    context.Background(),
		key:    defaultQueueKey,
	}
}

func NewQueueWithKey(redisClient *redis.Client, key string) *Queue {
	if redisClient == nil {
		panic("redis client cannot be nil")
	}
	if key == "" {
		key = defaultQueueKey
	}
	return &Queue{
		client: redisClient,
		ctx:    context.Background(),
		key:    key,
	}
}

func (q *Queue) Enqueue(req *RetryRequest) error {
	// Сериализуем запрос в JSON
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	// Используем RetryAt.Unix() как score для sorted set
	// Это позволяет эффективно получать элементы, готовые к повторной попытке
	score := float64(req.RetryAt.Unix())

	// Используем ID как member, чтобы можно было обновлять запросы
	member := req.ID

	// Добавляем в sorted set
	err = q.client.ZAdd(q.ctx, q.key, redis.Z{
		Score:  score,
		Member: member,
	}).Err()
	if err != nil {
		return err
	}

	// Сохраняем данные запроса в hash для быстрого доступа
	hashKey := q.key + ":data:" + req.ID
	err = q.client.Set(q.ctx, hashKey, data, 0).Err()
	if err != nil {
		// Если не удалось сохранить данные, удаляем из sorted set
		q.client.ZRem(q.ctx, q.key, member)
		return err
	}

	return nil
}

func (q *Queue) Dequeue() *RetryRequest {
	now := time.Now()
	nowUnix := float64(now.Unix())

	// Получаем все элементы с score <= nowUnix (готовые к повторной попытке)
	// Используем ZRANGEBYSCORE с LIMIT 0 1 для получения первого элемента
	members, err := q.client.ZRangeByScore(q.ctx, q.key, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   strconv.FormatFloat(nowUnix, 'f', -1, 64),
		Count: 1,
	}).Result()
	if err != nil {
		return nil
	}

	if len(members) == 0 {
		return nil
	}

	member := members[0]

	// Получаем данные запроса из hash
	hashKey := q.key + ":data:" + member
	data, err := q.client.Get(q.ctx, hashKey).Result()
	if err != nil {
		// Если данные не найдены, удаляем из sorted set
		q.client.ZRem(q.ctx, q.key, member)
		return nil
	}

	// Десериализуем запрос
	var req RetryRequest
	err = json.Unmarshal([]byte(data), &req)
	if err != nil {
		// Если не удалось десериализовать, удаляем из sorted set и hash
		q.client.ZRem(q.ctx, q.key, member)
		q.client.Del(q.ctx, hashKey)
		return nil
	}

	// Удаляем из sorted set и hash
	q.client.ZRem(q.ctx, q.key, member)
	q.client.Del(q.ctx, hashKey)

	return &req
}

func (q *Queue) Peek() *RetryRequest {
	now := time.Now()
	nowUnix := float64(now.Unix())

	// Получаем первый элемент, готовый к повторной попытке
	members, err := q.client.ZRangeByScore(q.ctx, q.key, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   strconv.FormatFloat(nowUnix, 'f', -1, 64),
		Count: 1,
	}).Result()
	if err != nil {
		return nil
	}

	if len(members) == 0 {
		return nil
	}

	member := members[0]

	// Получаем данные запроса из hash
	hashKey := q.key + ":data:" + member
	data, err := q.client.Get(q.ctx, hashKey).Result()
	if err != nil {
		return nil
	}

	// Десериализуем запрос
	var req RetryRequest
	err = json.Unmarshal([]byte(data), &req)
	if err != nil {
		return nil
	}

	return &req
}

func (q *Queue) Size() int {
	count, err := q.client.ZCard(q.ctx, q.key).Result()
	if err != nil {
		return 0
	}
	return int(count)
}

func (q *Queue) GetAll() []*RetryRequest {
	// Получаем все элементы из sorted set
	members, err := q.client.ZRange(q.ctx, q.key, 0, -1).Result()
	if err != nil {
		return []*RetryRequest{}
	}

	result := make([]*RetryRequest, 0, len(members))
	for _, member := range members {
		hashKey := q.key + ":data:" + member
		data, err := q.client.Get(q.ctx, hashKey).Result()
		if err != nil {
			continue
		}

		var req RetryRequest
		err = json.Unmarshal([]byte(data), &req)
		if err != nil {
			continue
		}

		result = append(result, &req)
	}

	return result
}
