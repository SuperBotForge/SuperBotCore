<script lang="ts">
    let {value = $bindable()}: {
        value: number // nanoseconds
    } = $props()

    let days: number = $state(0)
    let hours: number = $state(0)
    let minutes: number = $state(0)

    $effect(() => {
        const seconds = minutes * 60 + hours * 3600 + days * 86400
        value = seconds * 1000000000
    })
</script>

<label for="days">Дней</label>
<input required type="number" id="days" bind:value={days}/>

<label for="hours">Часов</label>
<input required type="number" id="hours" bind:value={hours}/>

<label for="minutes">Минут</label>
<input required type="number" id="minutes" bind:value={minutes}/>
