package main

import "testing"

func TestCredentials_Valid(t *testing.T) {
	testUser := map[string]string{"test":"$2y$10$n0MZPvD3lqFlEGqbZzu7vuAvdIYn1qJJAriFmSpXq/HbOQZ1nup3a"}

	type fields struct {
		disableCaching bool
		htpasswd       map[string]string
	}
	type args struct {
		user     string
		password string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{name: "no htpasswd but user pw", fields: fields{
			disableCaching: false,
			htpasswd:       nil,
		}, args: args{
			user:     "test",
			password: "test",
		},want: false},

		{name: "no htpasswd no user pw nc", fields: fields{
			disableCaching: false,
			htpasswd:       nil,
		}, args: args{
			user:     "",
			password: "",
		},want: false},

		{name: "no htpasswd no user pw c", fields: fields{
			disableCaching: true,
			htpasswd:       nil,
		}, args: args{
			user:     "",
			password: "",
		},want: false},

		{name: "htpasswd no user pw", fields: fields{
			disableCaching: false,
			htpasswd: testUser,
		}, args: args{
			user:     "",
			password: "",
		},want: false},

		{name: "htpasswd user wrong pw", fields: fields{
			disableCaching: false,
			htpasswd: testUser,
		}, args: args{
			user:     "test",
			password: "test ",
		},want: false},

		{name: "htpasswd user correct pw uncached", fields: fields{
			disableCaching: false,
			htpasswd: testUser,
		}, args: args{
			user:     "test",
			password: "test",
		},want: true},

		{name: "htpasswd user correct pw cached", fields: fields{
			disableCaching: false,
			htpasswd: testUser,
		}, args: args{
			user:     "test",
			password: "test",
		},want: true},

		{name: "htpasswd user correct pw cached but wrong pw", fields: fields{
			disableCaching: false,
			htpasswd: testUser,
		}, args: args{
			user:     "test",
			password: "test1",
		},want: false},

	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Credentials{
				disableCaching: tt.fields.disableCaching,
				htpasswd:       tt.fields.htpasswd,
			}
			if got := s.Valid(tt.args.user, tt.args.password); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}
