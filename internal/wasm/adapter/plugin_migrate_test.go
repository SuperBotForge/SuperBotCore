package adapter

import "testing"

func TestPluginMigrationTableName(t *testing.T) {
	tests := map[string]string{
		"teacher-absence":  "_goose_plugin_teacher_absence",
		"student.location": "_goose_plugin_student_location",
		" Practice ":       "_goose_plugin_practice",
		"---":              "_goose_plugin_plugin",
	}

	for input, want := range tests {
		if got := pluginMigrationTableName(input); got != want {
			t.Fatalf("pluginMigrationTableName(%q) = %q, want %q", input, got, want)
		}
	}
}
