package lxd

import (
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/lxc/lxd/shared/api"
	"log"
)

func resourceLxdProject() *schema.Resource {
	return &schema.Resource{
		Create: resourceLxdProjectCreate,
		Update: resourceLxdProjectUpdate,
		Delete: resourceLxdProjectDelete,
		Exists: resourceLxdProjectExists,
		Read:   resourceLxdProjectRead,
		Importer: &schema.ResourceImporter{
			State: resourceLxdProjectImport,
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
		},
	}
}

func resourceLxdProjectCreate(d *schema.ResourceData, meta interface{}) error {
	p := meta.(*lxdProvider)
	server, err := p.GetInstanceServer(p.selectRemote(d))
	if err != nil {
		return err
	}

	name := d.Get("name").(string)
	description := d.Get("description").(string)
	config := resourceLxdConfigMap(d.Get("config"))

	req := api.ProjectsPost{Name: name}
	req.Config = config
	req.Description = description

	if err := server.CreateProject(req); err != nil {
		return err
	}

	d.SetId(name)

	return resourceLxdProjectRead(d, meta)
}

func resourceLxdProjectUpdate(d *schema.ResourceData, meta interface{}) error {
	p := meta.(*lxdProvider)
	server, err := p.GetInstanceServer(p.selectRemote(d))
	if err != nil {
		return err
	}

	name := d.Id()

	var changed bool

	project, etag, err := server.GetProject(name)
	if err != nil {
		return err
	}

	// Copy the current project config into de updateble project struct
	newProject := api.ProjectPut{
		Config:      project.Config,
		Description: project.Description,
	}

	if d.HasChange("description") {
		changed = true
		_, newDescription := d.GetChange("description")
		newProject.Description = newDescription.(string)
	}

	if d.HasChange("config") {
		changed = true
		_, newConfig := d.GetChange("config")
		newProject.Config = resourceLxdConfigMap(newConfig)
	}

	if changed {
		err := server.UpdateProject(name, newProject, etag)
		if err != nil {
			return err
		}
	}

	return resourceLxdProjectRead(d, meta)
}

func resourceLxdProjectDelete(d *schema.ResourceData, meta interface{}) error {
	p := meta.(*lxdProvider)
	server, err := p.GetInstanceServer(p.selectRemote(d))
	if err != nil {
		return err
	}

	name := d.Id()

	return server.DeleteProject(name)
}

func resourceLxdProjectExists(d *schema.ResourceData, meta interface{}) (exists bool, err error) {
	p := meta.(*lxdProvider)
	server, err := p.GetInstanceServer(p.selectRemote(d))
	if err != nil {
		return false, err
	}

	name := d.Id()

	exists = false

	project, _, err := server.GetProject(name)
	if err == nil && project != nil {
		exists = true
	}

	return
}

func resourceLxdProjectRead(d *schema.ResourceData, meta interface{}) error {
	p := meta.(*lxdProvider)
	server, err := p.GetInstanceServer(p.selectRemote(d))
	if err != nil {
		return err
	}

	name := d.Id()

	project, _, err := server.GetProject(name)
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] Retrieved project #{name}: #{project}")

	d.Set("description", project.Description)
	d.Set("config", project.Config)
	return nil
}

func resourceLxdProjectImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	p := meta.(*lxdProvider)
	remote, name, err := p.LXDConfig.ParseRemote(d.Id())

	if err != nil {
		return nil, err
	}

	d.SetId(name)
	if p.LXDConfig.DefaultRemote != remote {
		d.Set("remote", remote)
	}

	server, err := p.GetInstanceServer(p.selectRemote(d))
	if err != nil {
		return nil, err
	}

	project, _, err := server.GetProject(name)
	if err != nil {
		return nil, err
	}

	log.Printf("[DEBUG] Import Retrieved project %s: %#v", name, project)

	d.Set("name", name)
	return []*schema.ResourceData{d}, nil
}
