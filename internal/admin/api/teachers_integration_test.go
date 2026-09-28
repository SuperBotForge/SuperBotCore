package api

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"SuperBotGo/internal/auth/tsu"
	"SuperBotGo/internal/authz"
	"SuperBotGo/internal/authz/outbox"
	"SuperBotGo/internal/authz/tuples"
	"SuperBotGo/internal/model"
	"SuperBotGo/internal/user"
	"SuperBotGo/migrations"
	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	authzed "github.com/authzed/authzed-go/v1"
	"github.com/authzed/grpcutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Uses dedicated disposable PostgreSQL and SpiceDB instances, never production.
// PostgreSQL gets a private schema; SpiceDB is expected to be an empty test instance.
func TestTeacherGraphIntegration(t *testing.T) {
	dsn, endpoint := os.Getenv("TEACHER_TEST_DATABASE_URL"), os.Getenv("TEACHER_TEST_SPICEDB")
	if dsn == "" || endpoint == "" {
		t.Skip("set TEACHER_TEST_DATABASE_URL and TEACHER_TEST_SPICEDB for isolated integration tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("teacher_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	db := stdlib.OpenDB(*cfg.ConnConfig)
	defer db.Close()
	migrator, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrator.UpTo(ctx, 29); err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`TRUNCATE persons, global_users, faculties CASCADE; INSERT INTO faculties(id,code,name) VALUES(1,'fit','Faculty');
 INSERT INTO departments(id,faculty_id,code,name) VALUES(1,1,'pi_master','Department'),(2,1,'other','Other');
 INSERT INTO global_users(id,role) VALUES(101,'USER'),(102,'USER'),(103,'USER');
 INSERT INTO persons(id,external_id,last_name,first_name,global_user_id) VALUES(201,'account-before','Teacher','Existing',101);
 INSERT INTO teacher_positions(person_id,department_id,position_title) VALUES(201,1,'Professor')`)
	if _, err := migrator.Up(ctx); err != nil {
		t.Fatal(err)
	}
	client, err := authzed.NewClient(endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()), grpcutil.WithInsecureBearerToken("local-teacher-test"))
	if err != nil {
		t.Fatal(err)
	}
	schemaBytes, err := os.ReadFile("../../../deployments/schema.zed")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.WriteSchema(ctx, &v1.WriteSchemaRequest{Schema: string(schemaBytes)}); err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	worker := outbox.NewWorker(pool, tuples.NewWriter(client), logger)
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); _ = worker.Run(workerCtx) }()
	defer func() { cancel(); <-done }()
	authorizer := authz.NewAuthorizerWithTTL(authz.NewPgStore(pool), client, logger, 0, 0)
	check := func(id model.GlobalUserID, dept string, want bool) {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for {
			var pending int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM authz_outbox WHERE processed_at IS NULL`).Scan(&pending); err != nil {
				t.Fatal(err)
			}
			got, err := authorizer.EvalPolicy(ctx, `has_role("USER") && check("teacher","department","`+dept+`")`, id)
			if pending == 0 && err == nil && got == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("graph user=%d dept=%s got=%v want=%v pending=%d err=%v", id, dept, got, want, pending, err)
			}
			time.Sleep(30 * time.Millisecond)
		}
	}
	check(101, "pi_master", true) // migration repairs an existing position
	h := NewImportHandler(nil, pool)
	department := int64(1)
	req := manualTeacherRequest{CreatePersonRequest: CreatePersonRequest{ExternalID: "account-new", LastName: "Teacher", FirstName: "New"}, TeacherPositionRequest: TeacherPositionRequest{DepartmentID: &department, PositionTitle: "Lecturer", EmploymentType: "full_time", Status: "active"}}
	if err := h.createTeacher(ctx, req); err != nil {
		t.Fatal(err)
	}
	if err := h.createTeacher(ctx, req); err == nil {
		t.Fatal("duplicate position accepted")
	}
	check(102, "pi_master", false)
	linker := tsu.NewLinker(user.NewPgUserRepo(pool), nil, user.NewPersonAutoLinker(pool), logger)
	if err := linker.Link(ctx, 102, "account-new"); err != nil {
		t.Fatal(err)
	}
	check(102, "pi_master", true)
	if err := linker.Link(ctx, 102, "account-new"); err != nil {
		t.Fatal(err)
	} // repeated authorization
	var personID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM persons WHERE external_id='account-new' AND global_user_id=102`).Scan(&personID); err != nil {
		t.Fatal(err)
	}
	var posID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM teacher_positions WHERE person_id=$1`, personID).Scan(&posID); err != nil {
		t.Fatal(err)
	}
	store := NewPgPositionStore(pool)
	req.DepartmentID = new(int64)
	*req.DepartmentID = 2
	if err := store.UpdateTeacherPosition(ctx, posID, req.TeacherPositionRequest); err != nil {
		t.Fatal(err)
	}
	check(102, "pi_master", false)
	check(102, "other", true)
	req.Status = "ended"
	if err := store.UpdateTeacherPosition(ctx, posID, req.TeacherPositionRequest); err != nil {
		t.Fatal(err)
	}
	check(102, "other", false)
	req.Status = "active"
	if err := store.UpdateTeacherPosition(ctx, posID, req.TeacherPositionRequest); err != nil {
		t.Fatal(err)
	}
	check(102, "other", true)
	secondReq := req.TeacherPositionRequest
	secondReq.PositionTitle = "Second appointment"
	second, err := store.CreateTeacherPosition(ctx, personID, secondReq)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteTeacherPosition(ctx, posID); err != nil {
		t.Fatal(err)
	}
	check(102, "other", true) // another active position keeps access
	if err := store.DeleteTeacherPosition(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	check(102, "other", false)
	// Authorization first, creation second also links the person.
	if err := linker.Link(ctx, 103, "account-later"); err != nil {
		t.Fatal(err)
	}
	req.ExternalID = "account-later"
	req.FirstName = "Later"
	if err := h.createTeacher(ctx, req); err != nil {
		t.Fatal(err)
	}
	check(103, "other", true)
	// Numeric bot IDs must not impersonate a different university identity.
	exec(`INSERT INTO global_users(id,role) VALUES(777,'USER'); INSERT INTO persons(id,external_id,last_name,first_name) VALUES(777,'777','Other','Person'); INSERT INTO teacher_positions(person_id,department_id,position_title) VALUES(777,1,'Lecturer')`)
	check(777, "pi_master", false)
	if err := tuples.NewWriter(client).WriteTuples(ctx, []tuples.Tuple{
		{ObjectType: "study_group", ObjectID: "test-group", Relation: "member", SubjectType: "user", SubjectID: "account-before"},
		{ObjectType: "system_role", ObjectID: "USER", Relation: "member", SubjectType: "user", SubjectID: "101"},
	}); err != nil {
		t.Fatal(err)
	}
	for _, expr := range []string{`is_member("study_group","test-group")`, `check("member","group","test-group")`, `check("is_member","system_role","USER")`} {
		if allowed, err := authorizer.EvalPolicy(ctx, expr, 101); err != nil || !allowed {
			t.Fatalf("%s: allowed=%v err=%v", expr, allowed, err)
		}
	}
	// Failed creation rolls back the person as well as the position.
	req.ExternalID = "invalid-department"
	*req.DepartmentID = 99999
	if err := h.createTeacher(ctx, req); err == nil {
		t.Fatal("invalid department accepted")
	}
	var invalidCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM persons WHERE external_id='invalid-department'`).Scan(&invalidCount); err != nil || invalidCount != 0 {
		t.Fatalf("partial creation: count=%d err=%v", invalidCount, err)
	}
	// A delayed retry must run without any new pg_notify event.
	exec(`INSERT INTO authz_outbox(operation,payload,locked_until) VALUES('SYNC_DEPARTMENT_STAFF','{"object_type":"department","object_id":"pi_master","relation":"staff"}',now()+interval '200 milliseconds')`)
	check(101, "pi_master", true)
	if _, err := authorizer.EvalPolicy(ctx, `check("missing_permission","department","pi_master")`, 101); err == nil {
		t.Fatal("SpiceDB errors were silently swallowed")
	}
}
