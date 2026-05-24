<script lang="ts">
    import {dateFormats} from "../constants";
    import moment from "moment-timezone";

    let {lesson, selected = true}: {
        lesson: Lesson
        selected: boolean
    } = $props()
</script>

<div class="card" class:selected>
    <div><b>{lesson.title}</b></div>
    <div>{lesson.lessonType}</div>
    <div>{lesson.groups.map(g => g.name).join(", ")}</div>
    <div>{lesson.professor}</div>
    <div>{lesson.audience}</div>
    <div><i>{moment.unix(lesson.startTime).format(dateFormats.HOUR_MINUTES)}-
        {moment.unix(lesson.endTime).format(dateFormats.HOUR_MINUTES)}</i></div>
</div>

<style>
    .card {
        border: 1px solid #d46b08;
        border-radius: 6px;
        padding: 2px 8px;
        font-size: 0.8em;
        text-align: start;
        margin: 6px;
    }

    .selected {
        color: var(--text-bright);
        background: #fbad57;
    }

    @media (prefers-color-scheme: dark) {
        .selected {
            background: #b35a06;
        }
    }
</style>