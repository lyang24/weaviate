package esvector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/elastic/go-elasticsearch/v5/esapi"
	"github.com/semi-technologies/weaviate/entities/schema/kind"
	"github.com/semi-technologies/weaviate/usecases/kinds"
)

type bulkControlObject struct {
	Index bulkIndex `json:"index"`
}

type bulkIndex struct {
	Index string `json:"_index"`
	ID    string `json:"_id"`
}

// warning: only use if all bulk requests are of type index, as this particular
// struct might not catch errors of other operations
type bulkIndexResponse struct {
	Errors bool       `json:"errors"`
	Items  []bulkItem `json:"items"`
}

type bulkItem struct {
	Index *bulkIndexItem `json:"index"`
}

type bulkIndexItem struct {
	Error interface{}
}

func (r *Repo) BatchPutActions(ctx context.Context, batch kinds.BatchActions) (kinds.BatchActions, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	r.encodeBatchActions(enc, batch)
	req := esapi.BulkRequest{
		Body: &buf,
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return nil, fmt.Errorf("batch put action request: %v", err)
	}

	if err := errorResToErr(res, r.logger); err != nil {
		return nil, fmt.Errorf("batch put action request: %v", err)
	}

	return mergeBatchActionsWithErrors(batch, res)
}

func (r Repo) encodeBatchActions(enc *json.Encoder, batch kinds.BatchActions) error {
	for _, single := range batch {
		if single.Err != nil {
			// ignore concepts that already have an error
			continue
		}

		a := single.Action
		bucket := r.objectBucket(kind.Action, a.ID.String(), a.Class, a.Schema,
			single.Vector, a.CreationTimeUnix, a.LastUpdateTimeUnix)

		index := classIndexFromClassName(kind.Action, a.Class)
		control := r.bulkIndexControlObject(index, a.ID.String())

		err := enc.Encode(control)
		if err != nil {
			return err
		}

		err = enc.Encode(bucket)
		if err != nil {
			return err
		}
	}

	return nil
}

func mergeBatchActionsWithErrors(batch kinds.BatchActions, res *esapi.Response) (kinds.BatchActions, error) {
	var parsed bulkIndexResponse
	err := json.NewDecoder(res.Body).Decode(&parsed)
	if err != nil {
		return nil, err
	}

	if !parsed.Errors {
		// no need to check for error positions if there are none
		return batch, nil
	}

	bulkIndex := 0
	for i, action := range batch {
		if action.Err != nil {
			// already had a validation error, was therefore never sent off to es
			continue
		}

		err := parsed.Items[bulkIndex].Index.Error
		if err != nil {
			batch[i].Err = fmt.Errorf("%v", err)
		}

		bulkIndex++
	}

	return batch, nil
}

func (r *Repo) BatchPutThings(ctx context.Context, batch kinds.BatchThings) (kinds.BatchThings, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	r.encodeBatchThings(enc, batch)
	req := esapi.BulkRequest{
		Body: &buf,
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return nil, fmt.Errorf("batch put thing request: %v", err)
	}

	if err := errorResToErr(res, r.logger); err != nil {
		return nil, fmt.Errorf("batch put thing request: %v", err)
	}

	return mergeBatchThingsWithErrors(batch, res)
}

func (r Repo) encodeBatchThings(enc *json.Encoder, batch kinds.BatchThings) error {
	for _, single := range batch {
		if single.Err != nil {
			// ignore concepts that already have an error
			continue
		}

		t := single.Thing
		bucket := r.objectBucket(kind.Thing, t.ID.String(), t.Class, t.Schema,
			single.Vector, t.CreationTimeUnix, t.LastUpdateTimeUnix)

		index := classIndexFromClassName(kind.Thing, t.Class)
		control := r.bulkIndexControlObject(index, t.ID.String())

		err := enc.Encode(control)
		if err != nil {
			return err
		}

		err = enc.Encode(bucket)
		if err != nil {
			return err
		}
	}

	return nil
}

func mergeBatchThingsWithErrors(batch kinds.BatchThings, res *esapi.Response) (kinds.BatchThings, error) {
	var parsed bulkIndexResponse
	err := json.NewDecoder(res.Body).Decode(&parsed)
	if err != nil {
		return nil, err
	}

	if !parsed.Errors {
		// no need to check for error positions if there are none
		return batch, nil
	}

	bulkIndex := 0
	for i, thing := range batch {
		if thing.Err != nil {
			// already had a validation error, was therefore never sent off to es
			continue
		}

		err := parsed.Items[bulkIndex].Index.Error
		if err != nil {
			batch[i].Err = fmt.Errorf("%v", err)
		}

		bulkIndex++
	}

	return batch, nil
}

func (r *Repo) bulkIndexControlObject(index, id string) bulkControlObject {
	return bulkControlObject{
		Index: bulkIndex{
			Index: index,
			ID:    id,
		},
	}
}
