package ingest

import (
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/stellar/go/support/db"
	"github.com/stellar/go/support/errors"
)

func (b *BatchInsertBuilder) init() {
	b.rows = make([][]interface{}, 0)
}

func (b *BatchInsertBuilder) createInsertBuilder() {
	b.insertBuilder = sq.Insert(string(b.TableName)).Columns(b.Columns...)
}

func (b *BatchInsertBuilder) GetAddresses() (adds []Address) {
	for _, row := range b.rows {
		for _, param := range row {
			if address, ok := param.(Address); ok {
				adds = append(adds, address)
			}
		}
	}
	return
}

func (b *BatchInsertBuilder) ReplaceAddressesWithIDs(mapping map[Address]int64) {
	for i := range b.rows {
		for j := range b.rows[i] {
			if address, ok := b.rows[i][j].(Address); ok {
				b.rows[i][j] = mapping[address]
			}
		}
	}
}

func (b *BatchInsertBuilder) Values(params ...interface{}) {
	b.initOnce.Do(b.init)
	b.rows = append(b.rows, params)
}

func (b *BatchInsertBuilder) Exec(DB *db.Session) error {
	b.initOnce.Do(b.init)
	b.createInsertBuilder()
	paramsCount := 0

	for _, row := range b.rows {
		b.insertBuilder = b.insertBuilder.Values(row...)
		paramsCount += len(row)

		// PostgreSQL supports up to 65535 parameters.
		if paramsCount > 65000 {
			_, err := DB.Exec(b.insertBuilder)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("Error adding values while inserting to %s", b.TableName))
			}
			paramsCount = 0
			b.createInsertBuilder()
		}
	}

	if paramsCount > 0 {
		_, err := DB.Exec(b.insertBuilder)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("Error adding values while inserting to %s", b.TableName))
		}
	}

	// Empty rows slice
	b.rows = make([][]interface{}, 0)
	return nil
}
