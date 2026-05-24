from selenium import webdriver
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.chrome.service import Service
from selenium.webdriver.chrome.webdriver import WebDriver
from webdriver_manager.chrome import ChromeDriverManager


def build_driver() -> WebDriver:
    options = Options()
    prefs = {
        'autofill.profile_enabled': False,
        'profile.password_manager_enabled': False,
    }
    options.add_experimental_option('prefs', prefs)
    # path of the chrome's profile parent directory - change this path as per your system
    options.add_argument(r"user-data-dir=C:\\Users\\User\\AppData\\Local\\Google\\Chrome\\User Data")
    # name of the directory - change this directory name as per your system
    options.add_argument("--profile-directory=Profile 5")
    driver = webdriver.Chrome(service=Service(ChromeDriverManager().install()), options=options)
    return driver
