import time

from selenium.webdriver import Keys
from selenium.webdriver.common.by import By
from selenium.webdriver.support.expected_conditions import url_to_be
from selenium.webdriver.support.wait import WebDriverWait

from common import build_driver

driver = build_driver()
driver.implicitly_wait(5)

driver.get("https://discord.com/login")

time.sleep(2)

pass_field = driver.find_element(By.NAME, "password")
pass_field.send_keys(Keys.RETURN)

WebDriverWait(driver, 30).until(url_to_be("https://discord.com/channels/@me"))

driver.quit()
