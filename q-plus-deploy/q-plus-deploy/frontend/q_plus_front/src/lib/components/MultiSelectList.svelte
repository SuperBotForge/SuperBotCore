<script lang="ts" generics="T">
    import type {Snippet} from 'svelte';

    let {
        list,
        group = $bindable(),
        getId,
        option
    }: {
        list: T[],
        group: number[] | undefined,
        getId: (item: T) => number,
        option: Snippet<[T]>
    } = $props()
</script>

{#each list as item}
    <div>
        <label>
            <input type="checkbox" value={getId(item)} bind:group={group}/>
            {@render option(item)}
        </label>
    </div>
{/each}