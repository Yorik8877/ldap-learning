package user_repo

import (
	"slices"
	"testing"
)

func TestConvertToDomainGroupNames(t *testing.T) {
	testCases := []struct {
		caseName   string
		memberOf   []string
		wantGroups []string
	}{
		{
			"several groups",
			[]string{
				"CN=PanelAdmins,OU=Groups,DC=corp,DC=example,DC=com",
				"CN=Domain Admins,CN=Users,DC=corp,DC=example,DC=com",
			},
			[]string{"PanelAdmins", "Domain Admins"},
		},
		{"no memberOf", nil, []string{}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.caseName, func(t *testing.T) {
			record := userRecord{Groups: testCase.memberOf}
			converted, err := record.convertToDomain()
			if err != nil {
				t.Fatalf("convertToDomain() error = %v", err)
			}
			if !slices.Equal(converted.Groups, testCase.wantGroups) {
				t.Fatalf("convertToDomain().Groups = %q, want %q", converted.Groups, testCase.wantGroups)
			}
		})
	}
}

// Пустой срез, а не nil: в JSON он станет "groups": [], а не "groups": null.
func TestConvertToDomainWithoutGroupsGivesEmptySlice(t *testing.T) {
	converted, err := (&userRecord{}).convertToDomain()
	if err != nil {
		t.Fatalf("convertToDomain() error = %v", err)
	}
	if converted.Groups == nil {
		t.Fatalf("convertToDomain().Groups = nil, want empty slice")
	}
}

func TestConvertToDomainRejectsMalformedGroupDN(t *testing.T) {
	record := userRecord{Groups: []string{"garbage"}}
	if _, err := record.convertToDomain(); err == nil {
		t.Fatalf("convertToDomain() with malformed memberOf DN: want error, got nil")
	}
}
