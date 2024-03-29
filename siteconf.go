package siteconf

import (
	"fmt"
	"strings"

	admin_plugin "github.com/ecletus-pkg/admin"
	"github.com/ecletus/db"
	"github.com/ecletus/plug"
	"github.com/ecletus/roles"
	path_helpers "github.com/moisespsena-go/path-helpers"
	"github.com/op/go-logging"
	"gopkg.in/mgo.v2/bson"

	"github.com/ecletus/admin"
	"github.com/ecletus/core"
	"github.com/go-aorm/aorm"
)

var log = logging.MustGetLogger(path_helpers.GetCalledDir())

type Plugin struct {
	plug.EventDispatcher
	db.DBNames
	admin_plugin.AdminNames
	SitesRegisterKey,
	SitesLoaderUID string
	res *admin.Resource
}

func (p *Plugin) RequireOptions() []string {
	return []string{p.SitesRegisterKey}
}

func (p *Plugin) Before() []string {
	return []string{p.SitesLoaderUID}
}

func (p *Plugin) After() []string {
	return []string{}
}

func (p *Plugin) OnRegister(options *plug.Options) {
	admin_plugin.Events(p).InitResources(func(e *admin_plugin.AdminEvent) {
		if p.res != nil {
			return
		}
		e.Admin.AddResource(&SiteConfigMain{}, &admin.Config{
			Singleton:  true,
			Virtual:    true,
			Permission: roles.AllowAny(admin.ROLE),
			Menu:       admin.MenuConfig,
			Param:      admin.ConfigParam + "/site",
			Setup: func(res *admin.Resource) {
				// menu := res.DefaultMenu()
				// menu.MdlIcon = "settings"
				// menu.Priority = -1
			},
		})
	})

	db.Events(p).DBOnMigrate(func(e *db.DBEvent) error {
		return e.AutoMigrate(&SiteConfig{}, &SiteConfigMain{}).Error
	})
}

func (p *Plugin) Init(options *plug.Options) {
	register := options.GetInterface(p.SitesRegisterKey).(*core.SitesRegister)
	get := func(site *core.Site, key interface{}) (value interface{}, ok bool) {
		if key, ok := key.(PrivateName); ok {
			var config SiteConfig
			if err := site.GetSystemDB().DB.First(&config, "id = ?", string(key)).Error; err == nil || aorm.IsRecordNotFoundError(err) {
				return config.Value, true
			} else {
				log.Errorf("load config for site %s failed: %v", site.Name(), err)
				return nil, true
			}
		}
		return
	}
	register.SiteConfigGetter.Append(core.NewSiteGetter(get, func(site *core.Site, key, dest interface{}) (ok bool) {
		var v interface{}
		if v, ok = get(site, key); !ok {
			return
		}
		s := v.(string)
		if s == "" {
			return
		}
		if err := bson.UnmarshalJSON([]byte(s), dest); err != nil {
			log.Errorf("unmarshal config for site %s into %T failed: %v", site.Name(), dest, err)
		}
		return
	}))
	register.SetSiteConfigSetterFactory(&privateSiteConfigSetterFactory{})
}

func Private(site *core.Site, key interface{}) (v string, ok bool) {
	var k PrivateName
	switch kt := key.(type) {
	case string:
		k = PrivateName(kt)
	case PrivateName:
		k = kt
	default:
		k = PrivateName(PrivateConfName(key))
	}
	if v, ok := site.Config().Get(k); ok {
		return v.(string), true
	}
	return
}

func MustPrivate(site *core.Site, key interface{}) (v string) {
	v, _ = Private(site, key)
	return
}

type PrivateName string

func (this PrivateName) Sub(sub string) PrivateName {
	return this + PrivateName("."+sub)
}
func (this PrivateName) Concat(sub string) PrivateName {
	return this + PrivateName(sub)
}

type privateSiteConfigSetterFactory struct {
	FactoryCallbacks []*core.SiteFactoryCallback
	sites            []*core.Site
}

func (this *privateSiteConfigSetterFactory) FactoryCallback(cb ...*core.SiteFactoryCallback) {
	this.FactoryCallbacks = append(this.FactoryCallbacks, cb...)

	for _, site := range this.sites {
		for _, cb := range cb {
			if cb.Setup != nil {
				cb.Setup(site, site.ConfigSetter())
			}
		}
	}
}

func (this *privateSiteConfigSetterFactory) Factory(site *core.Site) (setter core.ConfigSetter) {
	setter = &core.DefaultConfigSetter{
		func(key, value interface{}) (err error) {
			return SetPrivate(site, key, value)
		},
		func() {
			defer func() {
				site = nil
			}()

			for i, s := range this.sites {
				if s == site {
					this.sites = append(this.sites[0:i], this.sites[i+1:]...)
				}
			}
			for _, cb := range this.FactoryCallbacks {
				if cb.Destroy != nil {
					cb.Destroy(site)
				}
			}
		},
	}
	for _, cb := range this.FactoryCallbacks {
		if cb.Setup != nil {
			cb.Setup(site, setter)
		}
	}
	this.sites = append(this.sites, site)
	return
}

func SetPrivate(site *core.Site, key, value interface{}) (err error) {
	var k string
	switch kt := key.(type) {
	case string:
		k = kt
	case PrivateName:
		k = string(kt)
	case fmt.Stringer:
		k = kt.String()
	default:
		k = PrivateConfName(key)
	}
	cfg := &SiteConfig{
		ID: string(k),
	}
	var b []byte
	if b, err = bson.MarshalJSON(value); err != nil {
		return
	}
	cfg.Value = strings.TrimSpace(string(b))
	DB := site.GetSystemDB().DB.Model(&cfg)

	if err = DB.Save(cfg).Error; err != nil {
		log.Errorf("set private config %q for site %s failed: %v", k, site.Name(), err)
		return
	}
	return
}

func SetPrivateMap(site *core.Site, data map[interface{}]interface{}) (err error) {
	DB := site.GetSystemDB().DB.Model(&SiteConfig{}).Begin()
	defer func() {
		if err == nil {
			err = DB.Commit().Error
		} else {
			DB.Rollback()
		}
	}()
	for key, value := range data {
		var k string
		switch kt := key.(type) {
		case string:
			k = kt
		case PrivateName:
			k = string(kt)
		case fmt.Stringer:
			k = kt.String()
		default:
			k = PrivateConfName(key)
		}
		cfg := &SiteConfig{
			string(k),
			fmt.Sprint(value),
		}

		if db2 := DB.Update(cfg); err == nil && db2.RowsAffected == 1 {
			continue
		} else if err != nil {
			err = db2.Error
		} else if db2.RowsAffected == 0 {
			if err = DB.Create(cfg).Error; err == nil {
				continue
			}
			err = fmt.Errorf("set private config %q for site %s failed: %v", k, site.Name(), err)
			return
		}
	}
	return
}
