<script lang="ts">
    import RelativeTimePicker from "../lib/components/RelativeTimePicker.svelte";
    import {
        type CourseInstance, CourseInstanceApi,
        type CreateQueueTemplateRequest, MarkTableTabApi,
        type QueueTemplate,
        QueueTemplateApi
    } from "../lib/api-client";
    import {SvelteDate} from "svelte/reactivity";
    import {apiConf} from "../lib/ApiUtils";
    import MultiSelectList from "../lib/components/MultiSelectList.svelte";
    import ScheduleQueues from "./ScheduleQueues.svelte";

    let {parentCourseInstance, copyFromTemplate}: {
        parentCourseInstance: CourseInstance,
        copyFromTemplate?: QueueTemplate
    } = $props()

    const templateApi = new QueueTemplateApi(apiConf())
    const courseInstanceApi = new CourseInstanceApi(apiConf())
    const markTableTabApi = new MarkTableTabApi(apiConf())

    async function enrichTemplate(template: QueueTemplate): Promise<QueueTemplate> {
        template.criteria = await templateApi.listQueueTemplateCriteria({id: template.id})
        template.markTableTab = await templateApi.readQueueTemplateMarkTableTab({id: template.id})
        return template
    }

    let newTemplate: CreateQueueTemplateRequest = $state({
        name: "",
        courseInstance: parentCourseInstance.id,
        createdAt: new SvelteDate(),
        updatedAt: new SvelteDate(),
        markTableTab: -1,
        signUpLeadTime: 0,
        criteria: parentCourseInstance.criteria?.map(c => c.id) ?? []
    })

    $effect(() => {
        if (copyFromTemplate) {
            enrichTemplate(copyFromTemplate).then(template => {
                newTemplate.name = template.name + " (копия)"
                newTemplate.markTableTab = template.markTableTab.id
                newTemplate.signUpLeadTime = template.signUpLeadTime
                newTemplate.criteria = template.criteria?.map(c => c.id)
            })
        }
    })

    async function createTemplate(event: Event) {
        event.preventDefault()

        console.log("createTemplate", $state.snapshot(newTemplate))

        if (!newTemplate.name) {
            throw new Error("Введите название")
        }

        const markTable = await courseInstanceApi.readCourseInstanceMarkTable({id: parentCourseInstance.id})

        const markTableTab = await markTableTabApi.createMarkTableTab({
            createMarkTableTabRequest: {
                name: newTemplate.name,
                markTable: markTable.id,
                createdAt: new SvelteDate(),
                updatedAt: new SvelteDate(),
                sheetId: -1,
            }
        })

        newTemplate.markTableTab = markTableTab.id

        const resp = await templateApi.createQueueTemplate({createQueueTemplateRequest: newTemplate})
        console.log("createTemplate", resp)
        resp.markTableTab = markTableTab
        resp.markTableTab.markTable = markTable
        return resp
    }

    let result: Promise<QueueTemplate> | undefined = $state()


    // $inspect(newTemplate.criteria)
</script>

<section>
    <h4>Новый шаблон</h4>
    <form onsubmit={(e)=>{result = createTemplate(e)}}>
        <label for="templateName">Название</label>
        <input type="text" minlength="2" id="templateName" required bind:value={newTemplate.name}>

        <label>Критерии</label>
        {#if (parentCourseInstance.criteria?.length ?? 0) === 0 }
            <p>Критериев нет</p>
            <br>
        {/if}
        <MultiSelectList list={parentCourseInstance.criteria ?? []}
                         getId={(item)=>item.id}
                         bind:group={newTemplate.criteria}>
            {#snippet option(item)}
                <span>{item.name}</span>
            {/snippet}
        </MultiSelectList>

        <p>
            <label>За сколько времени до начала очереди открывать запись</label>
            <br>
            <RelativeTimePicker bind:value={newTemplate.signUpLeadTime}/>
        </p>

        <button type="submit">Создать</button>
    </form>
    {#if result}
        {#await result}
            <p>Создание...</p>
        {:then template}
            <p>Шаблон создан</p>
            <p>
                <a target="_blank" rel="noopener noreferrer"
                   href="https://docs.google.com/spreadsheets/d/{template.markTableTab.markTable.spreadsheetId}/edit#gid={template.markTableTab.sheetId}">
                    Перейти к шаблону
                </a>
            </p>

            <p>
                <ScheduleQueues queueTemplateId={template.id}/>
            </p>
        {:catch error}
            <p>Ошибка: {error.message}</p>
        {/await}
    {/if}
</section>
