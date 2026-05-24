<script lang="ts">

    import {SvelteDate} from "svelte/reactivity";
    import LessonCard from "../lib/components/LessonCard.svelte";
    import SelectableLesson from "../lib/components/SelectableLesson.svelte";
    import {
        DefaultApi,
        type Queue,
        QueueTemplateApi,
        type ScheduleQueuesRequestInner
    } from "../lib/api-client";
    import {apiConf} from "../lib/ApiUtils";
    import SelectList from "../lib/components/SelectList.svelte";
    import moment from "moment-timezone";
    import {getWeeksFromDays} from "../lib/date-utils";
    import {Link} from "svelte-routing";

    moment.locale('ru');
    moment.tz.setDefault("Asia/Novosibirsk");

    let {queueTemplateId}: {
        queueTemplateId: number
    } = $props()

    const defaultApi = new DefaultApi(apiConf())
    const queueTemplateApi = new QueueTemplateApi(apiConf())

    let queueTemplate = $derived.by(async () => {
        return await queueTemplateApi.readQueueTemplate({id: queueTemplateId})
    })

    let faculties: Promise<IntimeFaculty[]> = $state(getFaculties())

    async function getFaculties(): Promise<IntimeFaculty[]> {
        const response = await fetch("https://intime.tsu.ru/api/web/v1/faculties")
        const data = await response.json()
        const result: IntimeFaculty[] = data.map((faculty: any) => ({
            id: faculty.id,
            name: faculty.name,
        }))
        selectedFaculty = result.find(f => f.id === "aa30cf34-6279-11e9-8107-005056bc52bb") // HITs
        return result
    }

    let selectedFaculty: IntimeFaculty | undefined = $state()

    let groups: Promise<IntimeGroup[] | undefined> = $derived.by(async () => {
        if (!selectedFaculty) return undefined
        const response = await fetch(`https://intime.tsu.ru/api/web/v1/faculties/${selectedFaculty.id}/groups`)
        const data = await response.json()
        return data.map((group: any) => ({
            id: group.id,
            name: group.name,
        }))
    })

    let searchLessonsFilter: SearchFirstLessonFilter = $state({
        group: undefined,
        date: new SvelteDate().toISOString().split("T")[0],
    })

    let foundedLessons: Lesson[] | undefined = $state(undefined)

    async function searchLessons(event: Event) {
        event.preventDefault()

        if (!searchLessonsFilter.group) {
            alert("Выберите группу")
            return
        }

        const response = await fetch(`https://intime.tsu.ru/api/old-web/v1/schedule/group?id=${searchLessonsFilter.group?.id}&dateFrom=${searchLessonsFilter.date}&dateTo=${searchLessonsFilter.date}`)
        const schedule = await response.json()
        console.log("schedule", schedule)

        foundedLessons = schedule[0].lessons.filter((lesson: any) => lesson.type === "LESSON").map((lesson: any): Lesson => ({
            title: lesson.title,
            lessonType: lesson.lessonType,
            groups: lesson.groups,
            audience: lesson.audience.shortName,
            professor: lesson.professor.shortName,
            startTime: lesson.starts,
            endTime: lesson.ends,
            date: schedule[0].date,
        }))

        console.log("searchLessons", $state.snapshot(searchLessonsFilter))
    }

    let selectedLesson: Lesson | undefined = $state()

    const nextMonth = new SvelteDate()
    nextMonth.setDate(nextMonth.getDate() + 30)

    let filter: LessonsFilter = $state({
        title: "",
        lessonType: "",
        groups: [],
        startDate: new SvelteDate().toISOString().split("T")[0],
        endDate: nextMonth.toISOString().split("T")[0],
    })

    $effect(() => {
        if (selectedLesson) {
            filter.title = selectedLesson.title
            filter.lessonType = selectedLesson.lessonType
            filter.groups = selectedLesson.groups
        }
    })

    // $inspect(filter)

    let schedule: ScheduleWeek<Lesson>[] = $state([])

    async function getLessonsByFilter(event: Event) {
        event.preventDefault()

        const groups = filter.groups.map(g => "&groupIds=" + g.id).join("")
        const request = `https://intime.tsu.ru/api/old-web/v2/schedule/week?dateFrom=${filter.startDate}&dateTo=${filter.endDate}${groups}`
        const response = await fetch(request)
        const intimeSchedule = await response.json()
        console.log("allSchedule", intimeSchedule)

        const days: ScheduleDay<Lesson>[] = []

        intimeSchedule.map((day: any) => {
            day.lessons = day.lessons.filter((lesson: any) => {
                return lesson.title === filter.title && lesson.lessonType === filter.lessonType
            })
            return day
        })
            .filter((day: any) => day.lessons.length > 0)
            .forEach((day: any) => {
                days.push({
                    date: day.date,
                    items: day.lessons.map((lesson: any): Lesson => ({
                        title: lesson.title,
                        lessonType: lesson.lessonType,
                        groups: lesson.groups,
                        audience: lesson.audience.shortName,
                        professor: lesson.professor.shortName,
                        startTime: lesson.starts,
                        endTime: lesson.ends,
                        date: day.date,
                    }))
                })
            })
        console.log("days", days)

        schedule = getWeeksFromDays(days)
        selectedLessons = days.flatMap(day => day.items)
    }

    let selectedLessons: Lesson[] = $state([])

    $inspect(selectedLessons)

    async function scheduleQueues() {
        const lessonTimeToDateTime = (date: string, time: number): Date => {
            const unix = new SvelteDate(date).getTime() + time * 1000
            return new SvelteDate(unix)
        }

        const resp = await defaultApi.scheduleQueues({
            scheduleQueuesRequestInner: selectedLessons.map(
                (lesson: Lesson): ScheduleQueuesRequestInner => ({
                    queueTemplateId: queueTemplateId,
                    startTime: lessonTimeToDateTime(lesson.date, lesson.startTime),
                    endTime: lessonTimeToDateTime(lesson.date, lesson.endTime),
                })
            )
        })
        console.log("scheduleQueues", resp)
        return resp
    }

    let result: Promise<Queue[]> | undefined = $state()

</script>

{#await queueTemplate}
    <p>Загрузка...</p>
{:then queueTemplate}
    <h3>Планирование очередей по шаблону <span style="white-space: nowrap;">'{queueTemplate.name}'</span></h3>
    <section>
        <h4>Фильтр расписания</h4>
        <p>Выберите одну пару из расписания, для которой будут найдены повторяющиеся пары</p>
        <form onsubmit={searchLessons}>
            <label>Факультет</label>
            {#await faculties}
                <p>Загрузка...</p>
            {:then faculties}
                <SelectList list={faculties} bind:value={selectedFaculty}>
                    {#snippet option(item)}
                        {item.name}
                    {/snippet}
                </SelectList>
            {:catch error}
                <p>Ошибка: {error.message}</p>
            {/await}

            {#await groups}
                <p>Загрузка...</p>
            {:then groups}
                {#if groups}
                    <label for="searchGroup">Группа</label>
                    <SelectList list={groups} bind:value={searchLessonsFilter.group}>
                        {#snippet option(item)}
                            {item.name}
                        {/snippet}
                    </SelectList>
                {/if}
            {:catch error}
                <p>Ошибка: {error.message}</p>
            {/await}

            <label for="searchDate">Дата</label>
            <input type="date" id="searchDate" bind:value={searchLessonsFilter.date}>

            <button type="submit">Найти</button>
        </form>

        {#if foundedLessons !== undefined}
            {#if foundedLessons.length === 0}
                <p>Нет результатов</p>
            {/if}
            <div class="row" role="radiogroup">
                {#each foundedLessons as lesson}
                    <label>
                        <input type="radio" bind:group={selectedLesson} value={lesson}>
                        <LessonCard {lesson} selected={lesson===selectedLesson}/>
                    </label>
                {/each}
            </div>
        {/if}

        {#if selectedLesson}
            <h5>Фильтр для поиска регулярных пар: </h5>

            <form onsubmit={getLessonsByFilter}>
                <label for="filterTitle">Название</label>
                <input type="text" id="filterTitle" disabled bind:value={filter.title}>

                <label for="filterLessonType">Тип занятия</label>
                <input type="text" id="filterLessonType" disabled bind:value={filter.lessonType}>

                <label for="filterGroups">Группы</label>
                <input type="text" id="filterGroups" disabled value={filter.groups.map(g => g.name).join(", ")}>

                <label for="filterStartDate">Дата начала</label>
                <input type="date" id="filterStartDate" bind:value={filter.startDate}>

                <label for="filterEndDate">Дата окончания</label>
                <input type="date" id="filterEndDate" bind:value={filter.endDate}>

                <button type="submit">Поиск</button>
            </form>

            {#if schedule.length === 0}
                <p>Нет результатов</p>
            {:else}
                <h4>Результаты поиска</h4>
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
                    {#each schedule as week}
                        <tr>
                            <td>
                                <span style="white-space: nowrap;">{week.startDate}</span> -
                                <span style="white-space: nowrap;">{week.endDate}</span>
                            </td>
                            {#each week.days as day}
                                <td>
                                    {#each day.items as lesson}
                                        <SelectableLesson lesson={lesson} bind:group={selectedLessons}/>
                                    {/each}
                                </td>
                            {/each}
                        </tr>
                    {/each}
                    </tbody>
                </table>

                <p>Выбрано {selectedLessons.length} пар</p>
                <button onclick={()=>{result = scheduleQueues()}}>Запланировать очереди</button>

                {#if result}
                    {#await result}
                        <p>Создание...</p>
                    {:then queues}
                        <p>Создано {queues.length} очередей</p>

                        <Link to="/queues/{queueTemplateId}">Список очередей для шаблона '{queueTemplate.name}'</Link>
                    {:catch error}
                        <p>Ошибка: {error.message}</p>
                    {/await}
                {/if}
            {/if}
        {/if}


    </section>
{:catch error}
    <p>Ошибка: {error.message}</p>
{/await}

<style>
    .row {
        display: flex;
        flex-wrap: wrap;
    }

    input[type="text"] {
        width: 100%;
        box-sizing: border-box;
    }

    table {
        width: calc(100vw - 20px);
        margin-left: calc(-50vw + 50% + 10px);
    }
</style>