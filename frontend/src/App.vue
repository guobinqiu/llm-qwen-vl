<template>
  <div>
    <!-- 消息显示区域 -->
    <div>
      <div v-for="(msg, index) in showMessages" :key="index">
        {{ msg.role}}:{{ msg.content }}
      </div>
    </div>

    <!-- 聊天输入区域 -->
    <div 
      class="input-wrapper" 
      @dragover.prevent="handleDragOver" 
      @dragleave.prevent="handleDragLeave"
      @drop.prevent="handleDrop"
      :class="{ 'drag-over': isDragOver }"
    >
      <!-- 图片预览区域 -->
      <div class="image-preview" v-if="previewImages.length > 0">
        <div v-for="(img, index) in previewImages" 
          :key="index" 
          class="image-wrapper"
          :data-filename="img.filename"
          :data-hash="img.hash"
        >
          <img :src="img.url" />
          <span class="delete-btn" @click="removeImage(index)">×</span>
        </div>
      </div>
      
      <!-- 文本输入区域 -->
      <textarea 
        v-model="text"
        class="text-input" 
        placeholder="输入消息..."
        rows="1"
        @paste="handlePaste"
        ref="textInput"
      ></textarea>
      
      <!-- 拖拽提示 -->
      <div class="drop-zone" v-show="isDragOver">
        拖拽图片到这里上传
      </div>
      
      <button @click="sendMessage" class="send-button">发送</button>
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
      message: '',
      messages: [],
      previewImages: [], // 预览图片数组
      uploadedImages: new Set(), // 存储已上传图片的hash值
      isDragOver: false,
      ws: null,
    }
  },
  computed: {
    showMessages() {
      if (this.message.trim()) {
      return [...this.messages, { role: 'assistant', content: this.message }];
    }
    return this.messages;
    },
  },
  mounted() {
    this.initWebSocket()
  },
  beforeDestroy() {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.close()
      this.ws = null
    }
  },
  methods: {
    handlePaste(e) {
      const clipboard = e.clipboardData || window.clipboardData
      if (!clipboard) return

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
    
    handleDragOver() {
      this.isDragOver = true
    },
    
    handleDragLeave() {
      this.isDragOver = false
    },
    
    handleDrop(e) {
      this.isDragOver = false
      const dt = e.dataTransfer
      if (!dt) return

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
      if (this.previewImages.length >= this.maxImages) {
        alert(`最多只能上传${this.maxImages}张图片`)
        return
      }

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
        alert('上传出错: ' + err)
      }
    },
    insertImage(url, filename, fileHash) {
      this.previewImages.push({
        url: url,
        filename: filename,
        hash: fileHash
      })
      this.uploadedImages.add(fileHash)
      console.log('插入图片，当前数量:', this.previewImages.length)
    },
    
    removeImage(index) {
      const img = this.previewImages[index]
      if (img && img.hash) {
        this.uploadedImages.delete(img.hash)
        
        // 如果需要删除服务器上的图片，取消注释下面的代码
        /*
        try {
          axios.post('http://localhost:8080/delete-image', { filename: img.filename })
          console.log('已请求删除图片：', img.filename)
        } catch (err) {
          console.error('删除失败：', err)
        }
        */
      }
      
      this.previewImages.splice(index, 1)
      console.log('删除图片，当前数量:', this.previewImages.length)
    },
    
    getImageUrls() {
      return this.previewImages.map(img => {
        return img.url
      })
    },
    async sendMessage() {
      const text = this.text.trim()
      if (!text) return

      // 添加用户消息
      this.messages.push({role: 'user', content: text})
      
      // 发送到 WebSocket
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({
          content: text,
          images: this.getImageUrls(),
        }))
      }

      // 清空输入
      this.previewImages = []
      this.uploadedImages.clear()
      this.text = ''
    },
    initWebSocket() {
      this.ws = new WebSocket("ws://localhost:8080/chat")

      this.ws.onopen = () => {
        console.log("WebSocket 连接已打开")
      }

      this.ws.onmessage = (event) => {
        const chunk = event.data
        this.message += chunk

        if (chunk === '\n\n') {
          this.messages.push({role: 'assistant', content: this.message})
          this.message = ''
        }
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

<style scoped>
.input-wrapper {
  position: relative;
  border: 1px solid #ccc;
  border-radius: 6px;
  padding: 8px;
  min-height: 120px;
  background: white;
  transition: border-color 0.3s;
}

.input-wrapper.drag-over {
  border-color: #007bff;
  background: rgba(0, 123, 255, 0.05);
}

.image-preview {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 8px;
}

.image-wrapper {
  position: relative;
  display: inline-block;
}

.image-wrapper img {
  width: 100px;
  height: auto;
  border-radius: 6px;
  display: block;
}

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
  transition: background 0.2s;
}

.delete-btn:hover {
  background: rgba(0, 0, 0, 0.8);
}

.text-input {
  width: 100%;
  border: none;
  outline: none;
  font-size: 16px;
  line-height: 1.5;
  resize: none;
  background: transparent;
  min-height: 60px;
  font-family: inherit;
}

.send-button {
  margin-top: 10px;
  padding: 8px 16px;
  background: #007bff;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 14px;
  transition: background 0.2s;
}

.send-button:hover {
  background: #0056b3;
}

.drop-zone {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  border: 2px dashed #007bff;
  background: rgba(0, 123, 255, 0.1);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #007bff;
  font-weight: bold;
  border-radius: 4px;
  pointer-events: none;
}
</style>