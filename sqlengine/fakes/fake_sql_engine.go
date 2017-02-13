package fakes

import (
	"fmt"
	"github.com/AusDTO/pe-rds-broker/config"
)

type FakeSQLEngine struct {
	OpenCalled   bool
	OpenAddress  string
	OpenPort     int64
	OpenDBName   string
	OpenUsername string
	OpenPassword string
	OpenSSLMode  config.SSLMode
	OpenError    error

	CloseCalled bool

	ExistsDBCalled bool
	ExistsDBDBName string
	ExistsDBError  error

	CreateDBCalled bool
	CreateDBDBName string
	CreateDBError  error

	DropDBCalled bool
	DropDBDBName string
	DropDBError  error

	CreateUserCalled   bool
	CreateUserUsername string
	CreateUserPassword string
	CreateUserError    error

	DropUserCalled   bool
	DropUserUsername string
	DropUserError    error

	GrantPrivilegesCalled   bool
	GrantPrivilegesDBName   string
	GrantPrivilegesUsername string
	GrantPrivilegesError    error

	RevokePrivilegesCalled   bool
	RevokePrivilegesDBName   string
	RevokePrivilegesUsername string
	RevokePrivilegesError    error
}

func (f *FakeSQLEngine) Open(address string, port int64, dbname string, username string, password string, sslmode config.SSLMode) error {
	f.OpenCalled = true
	f.OpenAddress = address
	f.OpenPort = port
	f.OpenDBName = dbname
	f.OpenUsername = username
	f.OpenPassword = password
	f.OpenSSLMode = sslmode

	return f.OpenError
}

func (f *FakeSQLEngine) Close() {
	f.CloseCalled = true
}

func (f *FakeSQLEngine) ExistsDB(dbname string) (bool, error) {
	f.ExistsDBCalled = true
	f.ExistsDBDBName = dbname

	return true, f.ExistsDBError
}

func (f *FakeSQLEngine) CreateDB(dbname string) error {
	f.CreateDBCalled = true
	f.CreateDBDBName = dbname

	return f.CreateDBError
}

func (f *FakeSQLEngine) DropDB(dbname string) error {
	f.DropDBCalled = true
	f.DropDBDBName = dbname

	return f.DropDBError
}

func (f *FakeSQLEngine) CreateUser(username string, password string) error {
	f.CreateUserCalled = true
	f.CreateUserUsername = username
	f.CreateUserPassword = password

	return f.CreateUserError
}

func (f *FakeSQLEngine) DropUser(username string) error {
	f.DropUserCalled = true
	f.DropUserUsername = username

	return f.DropUserError
}

func (f *FakeSQLEngine) GrantPrivileges(dbname string, username string) error {
	f.GrantPrivilegesCalled = true
	f.GrantPrivilegesDBName = dbname
	f.GrantPrivilegesUsername = username

	return f.GrantPrivilegesError
}

func (f *FakeSQLEngine) RevokePrivileges(dbname string, username string) error {
	f.RevokePrivilegesCalled = true
	f.RevokePrivilegesDBName = dbname
	f.RevokePrivilegesUsername = username

	return f.RevokePrivilegesError
}

func (f *FakeSQLEngine) URI(dbname string, username string, password string) string {
	return fmt.Sprintf("fake://%s:%s@%s:%d/%s?reconnect=true", username, password, f.OpenAddress, f.OpenPort, dbname)
}

func (f *FakeSQLEngine) JDBCURI(dbname string, username string, password string) string {
	return fmt.Sprintf("jdbc:fake://%s:%d/%s?user=%s&password=%s", f.OpenAddress, f.OpenPort, dbname, username, password)
}

func (d *FakeSQLEngine) Address() string {
	return d.OpenAddress
}

func (d *FakeSQLEngine) Port() int64 {
	return d.OpenPort
}
