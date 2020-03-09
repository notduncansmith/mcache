package mcache

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/samsarahq/thunder/graphql"
	"github.com/samsarahq/thunder/graphql/graphiql"
	"github.com/samsarahq/thunder/graphql/introspection"
	"github.com/samsarahq/thunder/graphql/schemabuilder"
)

// UserStub isn't a thing
type UserStub struct{ ID string }

// buildSchema builds the graphql schema.
func (m *MCache) buildSchema() *graphql.Schema {
	schema := schemabuilder.NewSchema()
	schema.Object("Document", Document{})
	schema.Object("Manifest", Manifest{})
	schema.Object("UserStub", UserStub{})

	// schema.Object("IDSet", IDSet{})
	// schema.Object("DocSet", DocSet{})

	queries := schema.Query()
	queries.FieldFunc("whoami", func(args UserStub) UserStub {
		fmt.Println("WHO IS WHO?")
		return args
	})

	mutations := schema.Mutation()
	mutations.FieldFunc("echo", func(args struct{ Message string }) string {
		return args.Message
	})

	return schema.MustBuild()
}

// StartGraphQL starts the GraphQL server
func (m *MCache) StartGraphQL(addr string, includeGraphiQL bool) {
	schema := m.buildSchema()
	introspection.AddIntrospectionToSchema(schema)

	// Expose schema and graphiql.
	http.Handle("/graphql", graphql.HTTPHandler(schema))
	if includeGraphiQL {
		http.Handle("/graphiql/", http.StripPrefix("/graphiql/", graphiql.Handler()))
	}

	http.ListenAndServe(addr, nil)
}

func genericMap(v interface{}) map[string]interface{} {
	generic := map[string]interface{}{}
	bz, _ := json.Marshal(v)
	json.Unmarshal(bz, &generic)
	return generic
}
