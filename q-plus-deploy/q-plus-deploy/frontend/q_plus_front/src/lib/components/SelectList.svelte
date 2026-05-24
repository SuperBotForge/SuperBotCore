<script lang="ts" generics="T">
    import type {Snippet} from 'svelte';

    let {list, value = $bindable(), option}: {
        list: T[],
        value: T | undefined,
        option: Snippet<[T]>
    } = $props()
</script>

<select bind:value={value}>
    <option value={undefined}>-</option>
    {#each list as item}
        <option value={item}>
            {@render option(item)}
        </option>
    {/each}
</select>

<style>
    select {
        max-width: 100%;
    }
</style>