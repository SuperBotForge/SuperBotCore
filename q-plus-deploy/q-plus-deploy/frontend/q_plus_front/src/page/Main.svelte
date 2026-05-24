<script lang="ts">

    import {
        type CourseInstance,
        CourseInstanceApi,
        type QueueTemplate
    } from "../lib/api-client";
    import {apiConf} from "../lib/ApiUtils";
    import SelectList from "../lib/components/SelectList.svelte";
    import NewTemplate from "./NewTemplate.svelte";

    const courseApi = new CourseInstanceApi(apiConf())

    let {parentCourseInstanceId}: {
        parentCourseInstanceId: number
    } = $props()

    let parentCourseInstance = $derived.by(async () => {
        const course = await courseApi.readCourseInstance({id: parentCourseInstanceId})
        course.criteria = await courseApi.listCourseInstanceCriteria({id: parentCourseInstanceId})
        return course
    })

    let courseInstances = $state(courseApi.listCourseInstance())

    let copyFromCourseInstance: CourseInstance | undefined = $state()
    $inspect(copyFromCourseInstance)

    let copyFromTemplates = $derived.by(async () => {
        if (!copyFromCourseInstance) return []
        return await courseApi.listCourseInstanceQueueTemplates({id: copyFromCourseInstance.id})
    })

    let copyFromTemplate: QueueTemplate | undefined = $state()

</script>

{#await parentCourseInstance}
    <p>Загрузка...</p>
{:then parentCourseInstance}

    <h3>Создать новый шаблон очереди для предмета
        <span style="white-space: nowrap;">'{parentCourseInstance.name}'</span>
    </h3>

    <section>
        <h4>Скопировать из:</h4>

        {#await courseInstances}
            <p>Загрузка...</p>
        {:then courseInstances}
            <SelectList list={courseInstances} bind:value={copyFromCourseInstance}>
                {#snippet option(item)}
                    {item.name}
                {/snippet}
            </SelectList>
        {:catch error}
            <p>Ошибка: {error.message}</p>
        {/await}

        {#if copyFromCourseInstance}
            {#await copyFromTemplates}
                <p>Загрузка...</p>
            {:then selectedCourseTemplates}
                <SelectList list={selectedCourseTemplates} bind:value={copyFromTemplate}>
                    {#snippet option(item)}
                        {item.name}
                    {/snippet}
                </SelectList>
            {:catch error}
                <p>Ошибка: {error.message}</p>
            {/await}
        {/if}

        {#if copyFromTemplate}
            <div>
                Шаблон будет скопирован из {copyFromCourseInstance?.name} - {copyFromTemplate?.name}
            </div>
        {/if}
    </section>

    <NewTemplate {parentCourseInstance} {copyFromTemplate}/>
{:catch error}
    <p>Ошибка: {error.message}</p>
{/await}
