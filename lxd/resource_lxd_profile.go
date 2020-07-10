package lxd

import (
	"log"

	"github.com/hashicorp/terraform/helper/schema"

	"github.com/lxc/lxd/shared/api"
)

func resourceLxdProfile() *schema.Resource {
	return &schema.Resource{
		Create: resourceLxdProfileCreate,
		Update: resourceLxdProfileUpdate,
		Delete: resourceLxdProfileDelete,
		Exists: resourceLxdProfileExists,
		Read:   resourceLxdProfileRead,
		Importer: &schema.ResourceImporter{
			State: resourceLxdProfileImport,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				ForceNew: true,
				Required: true,
			},

			"description": {
				Type:     schema.TypeString,
				Optional: true,
			},

			"device": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},

						"type": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: resourceLxdValidateDeviceType,
						},

						"properties": {
							Type:     schema.TypeMap,
							Required: true,
						},
					},
				},
			},

			"config": {
				Type:     schema.TypeMap,
				Optional: true,
			},

			"remote": {
				Type:     schema.TypeString,
				ForceNew: true,
				Optional: true,
				Default:  "",
			},

			"project": {
				Type:     schema.TypeString,
				ForceNew: true,
				Optional: true,
				Default:  "",
			},
		},
	}
}

func resourceLxdProfileCreate(d *schema.ResourceData, meta interface{}) error {
	p := meta.(*lxdProvider)
	server, err := p.GetInstanceServer(p.selectRemote(d))
	if err != nil {
		return err
	}

	name := d.Get("name").(string)
	description := d.Get("description").(string)
	config := resourceLxdConfigMap(d.Get("config"))
	devices := resourceLxdDevices(d.Get("device"))
	project := d.Get("project").(string)

	req := api.ProfilesPost{Name: name}
	req.Config = config
	req.Devices = devices
	req.Description = description

	if project != "" {
		server = server.UseProject(project)
	}

	if err := server.CreateProfile(req); err != nil {
		return err
	}

	d.SetId(name)

	return resourceLxdProfileRead(d, meta)
}

func resourceLxdProfileRead(d *schema.ResourceData, meta interface{}) error {
	p := meta.(*lxdProvider)
	server, err := p.GetInstanceServer(p.selectRemote(d))
	if err != nil {
		return err
	}

	project := d.Get("project").(string)
	server = server.UseProject(project)

	name := d.Id()

	profile, _, err := server.GetProfile(name)
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Retrieved profile %s: %#v", name, profile)

	d.Set("description", profile.Description)
	d.Set("config", profile.Config)

	devices := make([]map[string]interface{}, 0)
	for name, lxddevice := range profile.Devices {
		device := make(map[string]interface{})
		device["name"] = name
		delete(lxddevice, "name")
		device["type"] = lxddevice["type"]
		delete(lxddevice, "type")
		device["properties"] = lxddevice
		devices = append(devices, device)
	}
	d.Set("device", devices)
	return nil
}

func resourceLxdProfileUpdate(d *schema.ResourceData, meta interface{}) error {
	p := meta.(*lxdProvider)
	server, err := p.GetInstanceServer(p.selectRemote(d))
	if err != nil {
		return err
	}

	name := d.Id()

	ioldProject, inewProject := d.GetChange("project")
	oldProject := ioldProject.(string)
	newProject := inewProject.(string)

	var changed bool

	if d.HasChange("project") {
		// Copy profile to new project if not yet exists in new project and
		// delete from old project if is not being used
		profile, _, err := server.UseProject(oldProject).GetProfile(name)
		if err != nil {
			return err
		}

		profileNames, err := server.UseProject(newProject).GetProfileNames()
		if err != nil {
			return err
		}

		profileExists := false
		for _, item := range profileNames {
			if item == name {
				profileExists = true
				break
			}
		}

		if !profileExists {
			err = server.UseProject(newProject).CreateProfile(
				api.ProfilesPost{
					ProfilePut: profile.ProfilePut,
					Name:       name,
				},
			)
			if err != nil {
				return err
			}
		}

		if len(profile.UsedBy) == 0 {
			// we can delete old profile because is not being used
			if err = server.UseProject(oldProject).DeleteProfile(name); err != nil {
				return err
			}
		}
	}

	server = server.UseProject(newProject)
	profile, etag, err := server.GetProfile(name)
	if err != nil {
		return err
	}

	// Copy the current profile config into the updatable profile struct.
	newProfile := api.ProfilePut{
		Config:      profile.Config,
		Description: profile.Description,
		Devices:     profile.Devices,
	}

	if d.HasChange("description") {
		changed = true
		_, newDescription := d.GetChange("description")
		newProfile.Description = newDescription.(string)
	}

	if d.HasChange("config") {
		changed = true
		_, newConfig := d.GetChange("config")
		newProfile.Config = resourceLxdConfigMap(newConfig)
	}

	if d.HasChange("device") {
		changed = true
		old, new := d.GetChange("device")
		oldDevices := resourceLxdDevices(old)
		newDevices := resourceLxdDevices(new)

		for n := range oldDevices {
			delete(newProfile.Devices, n)
		}

		for n, d := range newDevices {
			if n != "" {
				newProfile.Devices[n] = d
			}
		}

		log.Printf("[DEBUG] Updated device list: %#v", newProfile.Devices)
	}

	if changed {
		err := server.UpdateProfile(name, newProfile, etag)
		if err != nil {
			return err
		}
	}

	return resourceLxdProfileRead(d, meta)
}

func resourceLxdProfileDelete(d *schema.ResourceData, meta interface{}) (err error) {
	p := meta.(*lxdProvider)
	server, err := p.GetInstanceServer(p.selectRemote(d))
	if err != nil {
		return err
	}

	name := d.Id()
	project := d.Get("project").(string)

	return server.UseProject(project).DeleteProfile(name)
}

func resourceLxdProfileExists(d *schema.ResourceData, meta interface{}) (exists bool, err error) {
	p := meta.(*lxdProvider)
	server, err := p.GetInstanceServer(p.selectRemote(d))
	if err != nil {
		return false, err
	}

	name := d.Id()
	project := d.Get("project").(string)

	exists = false

	profile, _, err := server.UseProject(project).GetProfile(name)
	if err == nil && profile != nil {
		exists = true
	}

	return
}

func resourceLxdProfileImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	p := meta.(*lxdProvider)
	remote, name, err := p.LXDConfig.ParseRemote(d.Id())

	if err != nil {
		return nil, err
	}

	d.SetId(name)
	if p.LXDConfig.DefaultRemote != remote {
		d.Set("remote", remote)
	}

	project := d.Get("project").(string)
	server, err := p.GetInstanceServer(p.selectRemote(d))
	if err != nil {
		return nil, err
	}

	profile, _, err := server.UseProject(project).GetProfile(name)
	if err != nil {
		return nil, err
	}

	log.Printf("[DEBUG] Import Retrieved profile %s: %#v", name, profile)

	d.Set("name", name)
	return []*schema.ResourceData{d}, nil
}
