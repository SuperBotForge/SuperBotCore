<script lang="ts">

    import {type Queue, type QueueTemplate, QueueTemplateApi} from "../lib/api-client";
    import {apiConf} from "../lib/ApiUtils";
    import SelectableLesson from "../lib/components/SelectableLesson.svelte";
    import {getWeeksFromDays} from "../lib/date-utils";
    import moment from "moment-timezone";

    moment.locale('ru');
    moment.tz.setDefault("Asia/Novosibirsk");

    let {queueTemplateId}: { queueTemplateId: number } = $props()

    const templateApi = new QueueTemplateApi(apiConf())

    let queueTemplate = $derived.by(async () => {
        return await templateApi.readQueueTemplate({id: queueTemplateId})
    })

    let queues = $derived.by(async () => {
        return await templateApi.listQueueTemplateQueues({id: queueTemplateId})
    })

    let undatedQueues = $derived.by(async () => {
        if (!queues) return []
        return (await queues).filter(q => !q.startTime)
    })

    let datedQueues = $derived.by(async () => {
        if (!queues) return []
        const qs = (await queues).filter(q => q.startTime)

        const associateBy = <T, K>(arr: T[], f: (v: T) => K): Map<K, T[]> =>
            arr.reduce((acc, v) => {
                const key = f(v)
                const values = acc.get(key) ?? []
                values.push(v)
                acc.set(key, values)
                return acc
            }, new Map<K, T[]>)


        const days: ScheduleDay<Queue>[] = Array.from(associateBy(qs, (q): string => q.startTime?.toISOString().split('T')[0] ?? "").entries()).map(([date, queues]) => {
            return {
                date: date,
                items: queues
            }
        })
        console.log(days)
        return getWeeksFromDays(days)
    })

    $inspect(datedQueues)

</script>

<h2>Очереди</h2>

{#await queueTemplate}
    <p>Загрузка...</p>
{:then queueTemplate}
    <h3>Для шаблона '{queueTemplate.name}'</h3>

    <h4>Очереди без времени</h4>
    {#await undatedQueues}
        <p>Загрузка...</p>
    {:then undatedQueues}
        {#if undatedQueues.length === 0}
            <p>пусто</p>
        {:else}
            <ul>
                {#each undatedQueues as queue}
                    <li>{queue.name}</li>
                {/each}
            </ul>
        {/if}
    {:catch error}
        <p>Ошибка: {error.message}</p>
    {/await}

    <h4>Очереди с временем</h4>
    {#await datedQueues}
        <p>Загрузка...</p>
    {:then datedQueues}
        {#if datedQueues.length === 0}
            <p>пусто</p>
        {:else}
            <table>
                <thead>
                <tr>
                    <th></th>
                    <th>Пн</th>
                    <th>Вт</th>
                    <th>Ср</th>
                    <th>Чт</th>
                    <th>Пт</th>
                    <th>Сб</th>
                </tr>
                </thead>
                <tbody>
                {#each datedQueues as week}
                    <tr>
                        <td>
                            <span style="white-space: nowrap;">{week.startDate}</span> -
                            <span style="white-space: nowrap;">{week.endDate}</span>
                        </td>
                        {#each week.days as day}
                            <td>
                                <!-- TODO sort by time -->
                                {#each day.items as queue}
                                    <div class="card">
                                        <div><b>{queue.name}</b></div>
                                        <div>{moment(queue.startTime).format("hh:mm")}</div>
                                        <div>Критерии: {queue.criteria?.length ?? 0} шт</div>
                                        <div>Принимающих: {queue.examiners?.length ?? 0} шт</div>
                                        <div>Записей: {queue.places?.length ?? 0} шт</div>
                                    </div>
                                {/each}
                            </td>
                        {/each}
                    </tr>
                {/each}
                </tbody>
            </table>
        {/if}
    {:catch error}
        <p>Ошибка: {error.message}</p>
    {/await}
{/await}


<style>
    .card {
        border: 1px solid #d46b08;
        border-radius: 4px;
        padding: 2px 4px;
        font-size: 0.8em;
        text-align: start;
        margin: 4px;
    }

    table {
        width: calc(100vw - 20px);
        margin-left: calc(-50vw + 50% + 10px);
    }
</style>
