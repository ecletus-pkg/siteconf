package siteconf

import (
	"github.com/ecletus/fragment"
)

type SiteConfigMain struct {
	fragment.FragmentedModel
	Title string
}

type SiteConfig struct {
	ID    string `sql:"size:512;primary_key"`
	Value string `sql:"text"`
}

func (this SiteConfig) GetID() string {
	return this.ID
}

func (this *SiteConfig) SetID(value string) {
	this.ID = value
}
