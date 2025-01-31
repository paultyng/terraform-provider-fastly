package fastly

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/fastly/go-fastly/v9/fastly"
	gofastly "github.com/fastly/go-fastly/v9/fastly"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceFastlyKVStore() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceFastlyKVStoreCreate,
		ReadContext:   resourceFastlyKVStoreRead,
		UpdateContext: resourceFastlyKVStoreUpdate,
		DeleteContext: resourceFastlyKVStoreDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"force_destroy": {
				Type:        schema.TypeBool,
				Default:     false,
				Optional:    true,
				Description: "Allow the KV Store to be deleted, even if it contains entries. Defaults to false.",
			},
			// TODO: Move values to constants inside of go-fastly.
			"location": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The regional location of the KV Store. Valid values are `US`, `EU`, `ASIA`, and `AUS`.",
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice(
					[]string{"US", "EU", "ASIA", "AUS"},
					false,
				)),
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "A unique name to identify the KV Store. It is important to note that changing this attribute will delete and recreate the KV Store, and discard the current entries. You MUST first delete the associated resource_link block from your service before modifying this field.",
				ForceNew:    true,
			},
			"delete_keys_pool_size": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     100,
				Description: "Used only with `force_destroy` to define the size of the thread-pool used when deleting keys concurrently.",
			},
			"delete_keys_max_errors": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     100,
				Description: "Used only with `force_destroy` to define the buffer length of a channel holding any errors while deleting keys concurrently.",
			},
		},
	}
}

func resourceFastlyKVStoreCreate(_ context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	conn := meta.(*APIClient).conn

	input := &gofastly.CreateKVStoreInput{
		Name: d.Get("name").(string),
	}

	if v, ok := d.GetOk("location"); ok {
		input.Location = v.(string)
	}

	log.Printf("[DEBUG] CREATE: KV Store input: %#v", input)

	store, err := conn.CreateKVStore(input)
	if err != nil {
		return diag.FromErr(err)
	}

	// NOTE: `id` is exposed as a read-only attribute.
	d.SetId(store.StoreID)

	return nil
}

func resourceFastlyKVStoreRead(_ context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	conn := meta.(*APIClient).conn

	input := &gofastly.GetKVStoreInput{
		StoreID: d.Id(),
	}

	log.Printf("[DEBUG] REFRESH: KV Store input: %#v", input)

	store, err := conn.GetKVStore(input)
	if err != nil {
		if e, ok := err.(*gofastly.HTTPError); ok && e.IsNotFound() {
			log.Printf("[WARN] No KV Store found '%s'", d.Id())
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	err = d.Set("name", store.Name)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

// NOTE: There is no UPDATE endpoint for KV Stores.
func resourceFastlyKVStoreUpdate(_ context.Context, _ *schema.ResourceData, _ any) diag.Diagnostics {
	return nil
}

func resourceFastlyKVStoreDelete(_ context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	conn := meta.(*APIClient).conn

	storeEmpty, err := isKVStoreEmpty(d.Id(), conn)
	if err != nil {
		return diag.FromErr(err)
	}

	if !storeEmpty {
		if !d.Get("force_destroy").(bool) {
			return diag.FromErr(fmt.Errorf("cannot delete KV Store (%s), it is not empty. Either delete the entries first, or set force_destroy to true and apply it before making this change", d.Id()))
		}
		maxErrors := d.Get("delete_keys_max_errors").(int)
		poolSize := d.Get("delete_keys_pool_size").(int)
		err := deleteAllKVStoreKeys(conn, d.Id(), maxErrors, poolSize)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to delete all KV Store keys: %w", err))
		}
	}

	// p := conn.NewListKVStoreKeysPaginator(&gofastly.ListKVStoreKeysInput{
	// 	StoreID: d.Id(),
	// })
	// for p.Next() {
	// 	keys := p.Keys()
	// 	sort.Strings(keys)
	// 	for _, key := range keys {
	// 		err := conn.DeleteKVStoreKey(&gofastly.DeleteKVStoreKeyInput{
	// 			StoreID: d.Id(),
	// 			Key:     key,
	// 		})
	// 		if err != nil {
	// 			return diag.FromErr(fmt.Errorf("error during KV Store key cleanup: %w", err))
	// 		}
	// 	}
	// }
	// if err := p.Err(); err != nil {
	// 	return diag.FromErr(fmt.Errorf("error during KV Store cleanup pagination: %w", err))
	// }
	//

	input := gofastly.DeleteKVStoreInput{
		StoreID: d.Id(),
	}

	log.Printf("[DEBUG] DELETE: KV Store input: %#v", input)

	err = conn.DeleteKVStore(&input)
	if err != nil {
		return diag.FromErr(err)
	}
	return nil
}

// deleteAllKVStoreKeys deletes all keys within the specified KV Store.
func deleteAllKVStoreKeys(conn *gofastly.Client, storeID string, maxErrors, poolSize int) error {
	p := conn.NewListKVStoreKeysPaginator(&fastly.ListKVStoreKeysInput{
		StoreID: storeID,
	})

	errorsCh := make(chan string, maxErrors)
	keysCh := make(chan string, 1000) // number correlates to pagination page size

	var (
		failedKeys   []string
		mu           sync.Mutex
		wgProcessing sync.WaitGroup
		wgErrorCh    sync.WaitGroup
	)

	// We have three separate execution flows happening at once:
	//
	// 1. Pushing keys from pagination data into a key channel.
	// 2. Pulling keys from error channel and appending to failedKeys slice.
	// 3. Pulling keys from key channel and issuing API DELETE call.
	//
	// The second item is problematic, in that ranging over a channel only
	// terminates when the channel is closed. So we need to ensure we close the
	// errorsCh once we've finished processing the deletion of all the keys.
	//
	// To do that we need two sets of wait groups.
	//
	// The first is wgProcessing which keeps track of all goroutines related to
	// processing the pagination data (e.g. the goroutine ranging over the
	// paginator keys, and the goroutine ranging over the keysCh as part of the
	// poolSize loop).
	//
	// The second wait group is wgErrorCh which tracks when the first
	// (wgProcessing) has completed and then closes errorsCh.

	// The following goroutine finishes once all pagination keys have been
	// processed.
	wgProcessing.Add(1)
	go func() {
		defer wgProcessing.Done()
		defer close(keysCh)
		for p.Next() {
			for _, key := range p.Keys() {
				keysCh <- key
			}
		}
	}()

	// The following goroutine finishes once the errorsCh is closed.
	wgErrorCh.Add(1)
	go func() {
		defer wgErrorCh.Done()
		for err := range errorsCh {
			mu.Lock()
			failedKeys = append(failedKeys, err)
			mu.Unlock()
		}
	}()

	// The following goroutines close once they've pulled all data from keysCh.
	for i := 1; i <= poolSize; i++ {
		wgProcessing.Add(1)
		go func() {
			defer wgProcessing.Done()
			for key := range keysCh {
				err := conn.DeleteKVStoreKey(&fastly.DeleteKVStoreKeyInput{StoreID: storeID, Key: key})
				if err != nil {
					select {
					case errorsCh <- key:
					default:
						continue // the larger we make maxErrors the less likely we'll drop errors (obviously there's a memory trade-off to be made)
					}
				}
			}
		}()
	}

	// The following goroutine is closed once the 'processing' goroutines are
	// finished.
	wgErrorCh.Add(1)
	go func() {
		defer wgErrorCh.Done()
		wgProcessing.Wait() // Wait for all deletion and pagination tasks.
		close(errorsCh)
	}()

	// Wait for the error-handling goroutines to finish processing.
	wgErrorCh.Wait()

	if len(failedKeys) > 0 {
		return fmt.Errorf("failed to delete %d keys", len(failedKeys))
	}

	return nil
}

func isKVStoreEmpty(storeID string, conn *gofastly.Client) (bool, error) {
	keys, err := conn.ListKVStoreKeys(&gofastly.ListKVStoreKeysInput{
		StoreID: storeID,
	})
	if err != nil {
		return false, err
	}
	return len(keys.Data) == 0, nil
}
