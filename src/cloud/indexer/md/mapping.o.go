package md

import (
	"context"

	"github.com/olivere/elastic/v7"
)

// EsMDEntity is the struct that is stored in elastic.
type EsMDEntity struct {
	OrgID      string `json:"orgID"`
	ClusterUID string `json:"clusterUID"`
	VizierID   string `json:"vizierID"`
	UID        string `json:"uid"`
	Name       string `json:"name"`
	NS         string `json:"ns"`
	Kind       string `json:"kind"`

	TimeStartedNS int64 `json:"timeStartedNS"`
	TimeStoppedNS int64 `json:"timeStoppedNS"`

	RelatedEntityNames []string `json:"relatedEntityNames"`

	ResourceVersion string `json:"resourceVersion"`
}

// IndexMapping is the index structure for metadata entities.
const IndexMapping = `
{
    "settings":{
      "number_of_shards":1,
      "number_of_replicas":0,
        "analysis": {
          "filter": {
            "autocomplete_filter": {
              "type": "edge_ngram",
              "min_gram": 1,
              "max_gram": 20
            },
            "dont_split_on_numerics" : {
              "type" : "word_delimiter",
              "preserve_original": true,
              "generate_number_parts" : false
            }
          },
          "tokenizer": {
            "my_tokenizer": {
              "type": "pattern",
              "pattern": "-"
            },
            "ngram_tokenizer": {
              "type": "edge_ngram",
              "min_gram": 1,
              "max_gram": 20,
              "token_chars": ["letter", "digit"] 
            }
          },
          "analyzer": {
            "autocomplete": {
              "type": "custom",
              "tokenizer": "ngram_tokenizer",
              "filter": [
                "lowercase"
              ]
            },
            "myAnalyzer" : {
              "type" : "custom",
              "tokenizer" : "whitespace",
              "filter" : [ "dont_split_on_numerics" ]
            }
          }
        }
    },
  "mappings":{
    "properties":{
    "orgID":{
      "type":"text", "analyzer": "myAnalyzer"
    },
    "vizierID":{
      "type":"text", "analyzer": "myAnalyzer"
    },
    "clusterUID": {
      "type":"text", "analyzer": "myAnalyzer"
    },
    "uid":{
      "type":"text"
    },
    "name":{
      "type":"text",
        "analyzer": "autocomplete"
    },
    "ns":{
      "type":"text", "analyzer": "myAnalyzer"
    },
    "kind":{
      "type":"text"
    },
    "timeStartedNS":{
      "type":"long"
    },
    "timeStoppedNS":{
      "type":"long"
    },
    "relatedEntityNames":{
      "type":"text"
    },
    "ResourceVersion":{
      "type":"text"
    }
    }
  }
}
`

const indexName = "md_entities"

// InitializeMapping creates the index in elastic.
func InitializeMapping(es *elastic.Client) error {
	exists, err := es.IndexExists(indexName).Do(context.Background())
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = es.CreateIndex(indexName).Body(IndexMapping).Do(context.Background())
	return err
}