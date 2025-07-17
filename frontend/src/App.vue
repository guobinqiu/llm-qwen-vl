<template>
  <div id="app">
    <div
      class="input-box"
      contenteditable="true"
      ref="editor"
      @input="onInput"
      @paste="onPaste"
      @drop.prevent="onDrop"
      @dragover.prevent
    ></div>
    <button @click="sendContent">发送</button>

    <div v-if="replyMessage" class="reply-container">
      <p>{{ replyMessage }}</p>
    </div>
  </div>
</template>

<script>
import axios from 'axios'

export default {
  props: {
    maxImages: {
      type: Number,
      default: 1
    }
  },
  data() {
    return {
      text: '',
      uploadedImages: new Set(), // 存储已上传图片的hash值
      imageCount: 0,
      replyMessage: '',
      ws: null,
    }
  },
  mounted() {
    // 初始化编辑器结构
    this.initEditor()

    // 事件委托，监听图片上的删除按钮
    this.$refs.editor.addEventListener('click', (e) => {
      if (e.target.classList.contains('delete-btn')) {
        const wrapper = e.target.closest('.image-wrapper')
        if (wrapper) {
          const filename = wrapper.getAttribute('data-filename')
          const fileHash = wrapper.getAttribute('data-hash')

          this.deleteImage(filename)

          // 从已上传集合中移除hash值
          if (fileHash) {
            this.uploadedImages.delete(fileHash)
          }

          wrapper.remove()
          this.text = this.$refs.editor.innerHTML
        }
      }
    })

    this.initWebSocket()
  },
  destroyed() {
    if (this.ws) {
      this.ws.close()
    }
  },
  methods: {
    initEditor() {
      const editor = this.$refs.editor
      // 创建图片区域
      const imageArea = document.createElement('div')
      imageArea.className = 'image-area'

      // 创建文本区域
      const textArea = document.createElement('div')
      textArea.className = 'text-area'
      textArea.innerHTML = '<br>' // 添加一个换行符让光标可以定位

      editor.appendChild(imageArea)
      editor.appendChild(textArea)

      // 将光标定位到文本区域
      this.setCursorToTextArea()
    },
    setCursorToTextArea() {
      const textArea = this.$refs.editor.querySelector('.text-area')
      if (textArea) {
        textArea.focus()
        const range = document.createRange()
        const selection = window.getSelection()
        range.setStart(textArea, 0)
        range.collapse(true)
        selection.removeAllRanges()
        selection.addRange(range)
      }
    },
    onInput() {
      this.text = this.$refs.editor.innerHTML
    },
    onPaste(e) {
      const clipboard = e.clipboardData || window.clipboardData
      if (!clipboard) return

      // 检查图片数量限制
      if (this.imageCount > this.maxImages) {
        alert(`最多只能上传${this.maxImages}张图片`)
        return
      }

      for (const item of clipboard.items) {
        if (item.type.indexOf('image') !== -1) {
          const file = item.getAsFile()
          if (file) {
            e.preventDefault()
            this.uploadImage(file)
          }
        }
      }
    },
    onDrop(e) {
      const dt = e.dataTransfer
      if (!dt) return

      // 检查图片数量限制
      if (this.imageCount > this.maxImages) {
        alert(`最多只能上传${this.maxImages}张图片`)
        return
      }

      // 只上传符合条件的图片
      for (const file of dt.files) {
        if (file.type.startsWith('image/')) {
          this.uploadImage(file)
        }
      }
    },
    // 计算文件hash值
    async calculateFileHash(file) {
      return new Promise((resolve) => {
        const reader = new FileReader()
        reader.onload = async (e) => {
          const arrayBuffer = e.target.result
          const hashBuffer = await crypto.subtle.digest('SHA-256', arrayBuffer)
          const hashArray = Array.from(new Uint8Array(hashBuffer))
          const hashHex = hashArray.map(b => b.toString(16).padStart(2, '0')).join('')
          resolve(hashHex)
        }
        reader.readAsArrayBuffer(file)
      })
    },

    async uploadImage(file) {
      // 计算文件hash，用于去重
      const fileHash = await this.calculateFileHash(file)

      // 检查是否已存在相同图片
      if (this.uploadedImages.has(fileHash)) {
        alert('图片已存在，请勿重复上传')
        return
      }

      const formData = new FormData()
      formData.append('image', file)
      try {
        const res = await axios.post('http://localhost:8080/upload', formData, {
          headers: { 'Content-Type': 'multipart/form-data' }
        })
        if (res.data.errno === 0 && res.data.data.length > 0) {
          const url = res.data.data[0]
          const filename = url.split('/').pop()
          this.insertImage(url, filename, fileHash)
        } else {
          alert('上传失败')
        }
      } catch (err) {
        alert('上传出错', err)
      }
    },
    insertImage(url, filename, fileHash) {
      this.imageCount++

      // 在插入前再次检查数量限制（防止并发上传）
      if (this.imageCount > this.maxImages) {
        alert(`最多只能上传${this.maxImages}张图片`)
        return
      }

      const wrapper = document.createElement('span')
      wrapper.className = 'image-wrapper'
      wrapper.setAttribute('data-filename', filename)
      wrapper.setAttribute('data-hash', fileHash) // 存储hash值
      wrapper.style.display = 'inline-block'
      wrapper.style.position = 'relative'

      const img = document.createElement('img')
      img.src = url
      img.style.width = '100px'
      img.style.borderRadius = '6px'
      wrapper.appendChild(img)

      const delBtn = document.createElement('span')
      delBtn.className = 'delete-btn'
      delBtn.textContent = '×'
      wrapper.appendChild(delBtn)

      // 将图片插入到图片区域
      const imageArea = this.$refs.editor.querySelector('.image-area')
      imageArea.appendChild(wrapper)

      // 插入后立即打印一下 DOM 结构
      this.$nextTick(() => {
        const imageWrappers = imageArea.querySelectorAll('.image-wrapper')
        console.log('插入图片后的 DOM:', imageArea)
        console.log('当前图片数量:', imageWrappers.length)
        
        // 更新已上传图片的 hash
        this.uploadedImages.add(fileHash)

        // 更新文本内容
        this.text = this.$refs.editor.innerHTML

        // 插入图片后，将光标重新定位到文本区域
        this.setCursorToTextArea()
      })
    },
    async deleteImage(filename) {
      this.imageCount--
      if (this.imageCount < 0) {
        this.imageCount = 0
      }

      try {
        await axios.post('http://localhost:8080/delete-image', { filename })
        console.log('已请求删除图片：', filename)
      } catch (err) {
        console.error('删除失败：', err)
      }

      // 删除图片后，确保 DOM 完全更新后再继续检查和上传
      this.$nextTick(() => {
        // 在 DOM 更新后，再检查图片数量
        const imageArea = this.$refs.editor.querySelector('.image-area')
        const currentImageCount = imageArea ? imageArea.querySelectorAll('.image-wrapper').length : 0

        // 更新图片数量
        this.imageCount = currentImageCount

        // 打印当前图片数量，确保是正确的
        console.log('当前图片数量:', this.imageCount)
      })
    },
    getImageUrls() {
      const imageUrls = []
      const imageWrappers = this.$refs.editor.querySelectorAll('.image-wrapper')
      imageWrappers.forEach(wrapper => {
        const img = wrapper.querySelector('img')
        if (img) {
          // imageUrls.push(img.src)

          // 图片必须要公网可访问临时给一个
          imageUrls.push('https://pics5.baidu.com/feed/0bd162d9f2d3572c09e6decfee70572962d0c30a.jpeg')
        }
      })
      return imageUrls
    },
    async sendContent() {
      this.replyMessage = ''
      const message = {
        content: this.text,
        images: this.getImageUrls(),
      }
      
      // 确保 WebSocket 已经打开
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify(message)) // 发送内容和图片的消息
      } else {
        console.error("WebSocket 尚未连接或已关闭")
      }
    },
    initWebSocket() {
      this.ws = new WebSocket("ws://localhost:8080/chat")

      this.ws.onopen = () => {
        console.log("WebSocket 连接已打开");
      }

      this.ws.onmessage = (event) => {
        this.replyMessage += event.data
      }

      this.ws.onerror = (error) => {
        console.error("WebSocket 错误:", error)
      }

      this.ws.onclose = () => {
        console.log("WebSocket 连接已关闭")
      }
    },
  }
}
</script>

<style>
/* 图片计数器样式 */
.image-counter {
  margin-bottom: 8px;
  font-size: 14px;
  color: #666;
}

.input-box {
  border: 1px solid #ccc;
  min-height: 120px;
  max-width: 600px;
  margin: 0;
  padding: 8px;
  font-size: 16px;
  line-height: 1.5;
  outline: none;
  white-space: pre-wrap;
  word-break: break-word;
  border-radius: 6px;
}

/* 图片区域样式 */
.image-area {
  min-height: 0;
  margin-bottom: 8px;
}

/* 文本区域样式 */
.text-area {
  min-height: 20px;
  outline: none;
}

/* 包裹图片 + 删除按钮 */
.image-wrapper {
  position: relative;
  display: inline-block;
  margin: 6px;
}

/* 图片固定宽度 */
.image-wrapper img {
  width: 100px;
  height: auto;
  border-radius: 6px;
  display: block;
}

/* 删除按钮样式 */
.delete-btn {
  position: absolute;
  top: -6px;
  right: -6px;
  width: 18px;
  height: 18px;
  background: rgba(0, 0, 0, 0.6);
  color: white;
  font-size: 14px;
  line-height: 18px;
  text-align: center;
  border-radius: 50%;
  cursor: pointer;
  user-select: none;
}
</style>
