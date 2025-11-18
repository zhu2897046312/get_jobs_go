package locators

/**
 * Boss直聘网页元素定位器
 * 集中管理所有页面元素的定位表达式
 */

// 主页相关元素
const LOGIN_BTN = "//li[@class='nav-figure']"
const LOGIN_SCAN_SWITCH = "//div[@class='btn-sign-switch ewm-switch']"

/**
 * 搜索结果页相关元素
 */
// 用于判断岗位列表区块是否加载完成
const JOB_LIST_CONTAINER = "//div[@class='job-list-container']"
// 定位一个岗位卡
const JOB_CARD_BOX = "li.job-card-box"

/**
 * 岗位列表
 */
// 定位所有岗位卡片，用于获取当前获取到的岗位总数
const JOB_LIST_SELECTOR = "ul.rec-job-list li.job-card-box"
// 岗位名称
const JOB_NAME = "a.job-name"
// 公司名称
const COMPANY_NAME = "span.boss-name"
// 公司区域
const JOB_AREA = "span.company-location"
// 岗位标签
const TAG_LIST = "ul.tag-list li"

// 职位详情页元素
const CHAT_BUTTON = "[class*='btn btn-startchat']"
const ERROR_CONTENT = "//div[@class='error-content']"
const JOB_DETAIL_SALARY = "//div[@class='info-primary']//span[@class='salary']"
const RECRUITER_INFO = "//div[@class='boss-info-attr']"
const HR_ACTIVE_TIME = "//span[@class='boss-active-time']"
const JOB_DESCRIPTION = "//div[@class='job-sec-text']"

// 聊天相关元素
const DIALOG_TITLE = "//div[@class='dialog-title']"
const DIALOG_CLOSE = "//i[@class='icon-close']"
const CHAT_INPUT = "//div[@id='chat-input']"
const DIALOG_CONTAINER = "//div[@class='dialog-container']"
const SEND_BUTTON = "//button[@type='send']"
const IMAGE_UPLOAD = "//div[@aria-label='发送图片']//input[@type='file']"
const DIALOG_CONTENT = "//div[@class='dialog-con']"
const SCROLL_LOAD_MORE = "//div[contains(text(), '滚动加载更多')]"

// 消息列表页元素
const CHAT_LIST_ITEM = "//li[@role='listitem']"
const COMPANY_NAME_IN_CHAT = "//div[@class='title-box']/span[@class='name-box']//span[2]"
const LAST_MESSAGE = "//div[@class='gray last-msg']/span[@class='last-msg-text']"
const FINISHED_TEXT = "//div[@class='finished']"

const DIALOG_CON = ".dialog-con"
const LOGIN_BTNS = "//div[@class='btns']"
const PAGE_HEADER = "//h1"
const ERROR_PAGE_LOGIN = "//a[@ka='403_login']"