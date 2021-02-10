package zerotier

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/zerotier/go-ztcentral"
)

func resourceNetwork() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceNetworkCreate,
		ReadContext:   resourceNetworkRead,
		UpdateContext: resourceNetworkUpdate,
		DeleteContext: resourceNetworkDelete,
		Schema: map[string]*schema.Schema{
			"last_updated": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "Managed by Terraform",
			},
			"route": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"via": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"target": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"assignment_pool": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"start": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"end": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}
}

func resourceNetworkCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*ztcentral.Client)
	var diags diag.Diagnostics

	routes, err := mkRoutes(d.Get("route"))
	if err != nil {
		return diag.FromErr(err)
	}

	ipranges, err := mkIPRange(d.Get("assignment_pool"))
	if err != nil {
		return diag.FromErr(err)
	}

	n, err := c.NewNetwork(ctx, d.Get("name").(string), &ztcentral.NetworkConfig{
		IPAssignmentPool: ipranges,
		Routes:           routes,
		IPV4AssignMode:   ztcentral.IPV4AssignMode{ZeroTier: true},
		IPV6AssignMode:   ztcentral.IPV6AssignMode{ZeroTier: true},
	})

	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Unable to create ZeroTier Network",
			Detail:   fmt.Sprintf("CreateNetwork returned error: %v", err),
		})
		return diags
	}

	d.SetId(n.ID)

	resourceNetworkRead(ctx, d, m)
	return diags
}

func resourceNetworkRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*ztcentral.Client)
	var diags diag.Diagnostics

	ztNetworkID := d.Id()
	ztNetwork, err := c.GetNetwork(ctx, ztNetworkID)

	if err != nil {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  "Unable to read ZeroTier Network",
			Detail:   fmt.Sprintf("GetNetwork returned error: %v", err),
		})
		return diags
	}

	d.Set("name", ztNetwork.Config.Name)
	d.Set("description", ztNetwork.Description)
	d.Set("route", mktfRoutes(ztNetwork.Config.Routes))
	d.Set("assignment_pool", mktfRanges(ztNetwork.Config.IPAssignmentPool))

	return diags
}

func resourceNetworkUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*ztcentral.Client)
	var diags diag.Diagnostics

	ztNetworkID := d.Id()
	ztNetwork, err := c.GetNetwork(ctx, ztNetworkID)
	if err != nil {
		return diag.FromErr(err)
	}

	var changed bool

	if d.HasChange("description") {
		changed = true
		ztNetwork.Description = d.Get("description").(string)
	}

	if d.HasChange("route") {
		changed = true
		var err error
		ztNetwork.Config.Routes, err = mkRoutes(d.Get("route"))
		if err != nil {
			diags = append(diags, diag.FromErr(err)...)
		}
	}

	if d.HasChange("assignment_pool") {
		changed = true
		var err error
		ztNetwork.Config.IPAssignmentPool, err = mkIPRange(d.Get("assignment_pool"))
		if err != nil {
			diags = append(diags, diag.FromErr(err)...)
		}
	}

	if changed {
		if _, err := c.UpdateNetwork(ctx, ztNetwork); err != nil {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Unable to update ZeroTier Network description",
				Detail:   fmt.Sprintf("UpdateNetwork returned error: %v", err),
			})
			return diags
		}
		d.Set("last_updated", time.Now().Format(time.RFC850))
	}

	// return diags
	return resourceNetworkRead(ctx, d, m)
}

func resourceNetworkDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	c := m.(*ztcentral.Client)
	var diags diag.Diagnostics

	networkID := d.Id()

	err := c.DeleteNetwork(ctx, networkID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return diags
}
