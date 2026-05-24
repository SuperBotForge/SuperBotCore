import time
import random

import pyperclip
from selenium.webdriver import Keys
from selenium.webdriver.common.by import By
from selenium.webdriver.support.expected_conditions import visibility_of_element_located, element_to_be_clickable
from selenium.webdriver.support.wait import WebDriverWait

from common import build_driver

driver = build_driver()
driver.implicitly_wait(5)

queue_channel = "test-bot-1-queue"
queue_channel_id = "1238142935657222266"

student_channel = "test-bot-1-student"
student_channel_id = "1238178927017594963"

teacher_channel = "test-bot-1-teacher"
teacher_channel_id = "1238178767466401853"

driver.get("https://discord.com/channels/1039501019513638953/" + teacher_channel_id)
time.sleep(4)


def get_input_field(channel: str = teacher_channel):
    return driver.find_element(By.CSS_SELECTOR, f'div[aria-label*="#{channel}"][role="textbox"]')


def send_to_chat(text: str, channel: str = teacher_channel):
    input_field = get_input_field(channel)

    input_field.send_keys(text)
    input_field.send_keys(Keys.RETURN)


def send_from_clipboard(text: str, channel: str = teacher_channel):
    input_field = get_input_field(channel)

    pyperclip.copy(text)

    input_field.send_keys(Keys.SHIFT, Keys.INSERT)
    time.sleep(1)
    input_field.send_keys(Keys.RETURN)


def send_slash_command(command: str, channel: str = teacher_channel):
    input_field = get_input_field(channel)

    input_field.send_keys(command)
    time.sleep(2)
    input_field.send_keys(Keys.RETURN)
    time.sleep(0.5)
    input_field.send_keys(Keys.RETURN)


def check_last_message(message: str, channel: str = teacher_channel) -> bool:
    # message_list = driver.find_element(By.CSS_SELECTOR, f'ol[aria-label*="{channel}"][role="list"]')

    # messages = message_list.find_elements(By.CSS_SELECTOR, '#---new-messages-bar ~ li')
    last_message = driver.find_element(By.CSS_SELECTOR, f'ol[aria-label*="{channel}"][role="list"] > li:last-of-type')

    return message in last_message.text


def click_wizard_edit(channel: str = teacher_channel):
    button = driver.find_element(By.XPATH, "//ol/li[last()]//button[.//div[contains(., 'Редактировать')]]")
    WebDriverWait(driver, 5).until(element_to_be_clickable(button))
    time.sleep(2)
    button.click()


def input_wizard(message: str):
    textarea = driver.find_element(By.CSS_SELECTOR, "div[role=dialog] textarea")
    textarea.clear()
    textarea.send_keys(message)

    submit = driver.find_element(By.CSS_SELECTOR, "div[role=dialog] button[type=submit]")
    submit.click()


def create_queue(template_query: str, channel: str = teacher_channel):
    input_field = get_input_field(channel)

    input_field.send_keys("/queue create ")
    time.sleep(2)
    input_field.send_keys(template_query)
    time.sleep(2)
    input_field.send_keys(Keys.RETURN)
    time.sleep(0.5)
    input_field.send_keys(Keys.RETURN)


def queue_teacher_set(queue_query: str, channel: str = teacher_channel):
    input_field = get_input_field(channel)

    input_field.send_keys("/queue teacher set ")
    time.sleep(2)
    input_field.send_keys(queue_query)
    time.sleep(2)
    input_field.send_keys(Keys.RETURN)
    time.sleep(0.5)
    input_field.send_keys(Keys.RETURN)


def select_criteria(criterion_name: str):
    button = driver.find_element(By.XPATH, "//span[contains(., 'Выберите критерии')]")
    WebDriverWait(driver, 5).until(element_to_be_clickable(button))
    time.sleep(2)
    button.click()
    time.sleep(0.5)

    criterion = driver.find_element(By.XPATH, f"//div[@role='option' and contains(., '{criterion_name}')]")
    WebDriverWait(driver, 2).until(element_to_be_clickable(criterion))
    criterion.click()

    # press esc
    time.sleep(0.5)
    criterion.send_keys(Keys.ESCAPE)


# send_from_clipboard("/course create name:testing")
# time.sleep(7)
# if check_last_message("Предмет 'testing' создан"):
#     print("yay")
# else:
#     print("nope")

send_from_clipboard("/course criterion wizard ")
time.sleep(3)
click_wizard_edit()
WebDriverWait(driver, 6).until(visibility_of_element_located((By.CSS_SELECTOR, "div[role=dialog] textarea")))
time.sleep(1)
input_wizard("""
1. crit1
2. crit2
crit3""")

time.sleep(4)
if check_last_message("3. crit3"):
    print("yay")
else:
    print("nope")

a = random.randint(0, 1000000000000)
send_from_clipboard(
    f"/queue-template create name:test_template_{a} all_students_should_be_available:True sign_up_lead_time:1d "
)
time.sleep(6)
if check_last_message(f"Шаблон очереди 'test_template_{a}' создан"):
    print("yay")
else:
    print("nope")

# a = 548726192385
time.sleep(2)
create_queue(f"{a}")
queue_teacher_set(f"{a}")
select_criteria("crit1")
time.sleep(2)

# time.sleep(10)

driver.quit()
