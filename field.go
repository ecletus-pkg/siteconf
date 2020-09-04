package siteconf

import (
	"crypto/sha1"
	"fmt"
	"strings"

	"github.com/ecletus/core"

	"github.com/ecletus/admin"
	"github.com/ecletus/core/utils"
	"github.com/moisespsena-go/i18n-modular/i18nmod"
	path_helpers "github.com/moisespsena-go/path-helpers"
)

type Key uint8

const (
	OptFieldKey Key = iota
	OptFieldID
)

func GetFieldID(v interface{}) string {
	switch vt := v.(type) {
	case string:
		return fmt.Sprintf("%x", sha1.Sum([]byte(vt)))
	case *admin.Meta:
		return vt.Options.Value(OptFieldID).(string)
	default:
		panic("bad argument")
	}
}

func fieldEnabled(recorde interface{}, context *admin.Context, meta *admin.Meta) bool {
	return GetFieldID(meta) == context.ResourceID.String()
}

type FieldOptions struct {
	Key             interface{}
	Meta            *admin.Meta
	Getter          Getter
	FormattedGetter FormattedGetter
}

func (this FieldOptions) New() *admin.Meta {
	var name string
	if this.Meta == nil {
		this.Meta = &admin.Meta{}
		name = strings.TrimSuffix(utils.IndirectType(this.Key).Name(), "Key")
	} else {
		name = this.Meta.Name
	}

	this.Meta.Name = PrivateConfName(this.Key)
	this.Meta.Label = i18nmod.PkgToGroup(path_helpers.PkgPathOf(this.Key)) + "." + name
	this.Meta.Options.Set(OptFieldKey, this.Key)
	this.Meta.Options.Set(OptFieldID, GetFieldID(this.Meta.Name))
	this.Meta.Enabled = fieldEnabled
	this.Meta.Valuer = func(recorde interface{}, context *core.Context) interface{} {
		return this.Getter(context, recorde.(*SiteConfig).Value)
	}
	this.Meta.FormattedValuer = func(recorde interface{}, context *core.Context) interface{} {
		return this.FormattedGetter(context, this.Getter(context, recorde.(*SiteConfig).Value))
	}
	return this.Meta
}

func Field(opt *FieldOptions) *admin.Meta {
	return opt.New()
}

type Getter func(context *core.Context, value string) interface{}
type FormattedGetter func(context *core.Context, value interface{}) string
