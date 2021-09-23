
const ImgPreviewContainer = styled.div`
    transition: .3s ease-in-out all;
    transform: ${props => props.show ? 'translateY(0px)' : 'translateY(100px)'};
    height: ${props => props.show ? 'unset' : '0px'};
    border: 1px solid rgba(255, 255, 255, .2);
    padding-top: 8px;
    margin-right: 18px;
    padding-left: 12px;
    background: #27282c;
    border-radius: 6px;
    display: flex;
    flex-wrap: wrap;
`

const ImgPreview = styled.img`
    height: 60px;
    padding: 4px;
    margin: 3px;
    display: inline-block;
`

const SImgPreviewWrapper = styled.div`
    border: 1px dashed rgba(255, 255, 255, .5);
    position: relative;
    margin-right: 12px;
    padding-bottom: 8px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    align-self: center;
  span {
    display: inline-block;
    font-size: 10px;
    max-width: 120px;
    text-overflow: ellipsis;
    padding-left: 4px;
    padding-right: 4px;
    overflow: hidden;
    text-align: center;
    white-space: nowrap;
  }
    button {
      cursor: pointer;
      border: none;
      position: absolute;
      top: -4px;
      right: -4px;
      border-radius: 100%;
      height: 24px;
      width: 24px;
      display: flex;
      align-items: center;
      justify-content: center;
      background: ${props => props.theme.color.indigo[500]};
      transition: all ease-in-out .15s;
      outline: none;
      &:hover {
        transform: scale(1.1);
      }
      svg {
        color: white;
        height: 18px;
      }
    }
`

const SAddNewAttachment = styled.div`
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    margin-right: 12px;
    border: 1px dashed rgba(255,255,255,.5);
    max-height: 108.47px;
    margin: 3px;
    padding-left: 8px;
    padding-right: 8px;
    padding-bottom: 8px;
    > img {
        height: 60px;
        transform: scale(1.1);
    }
`


function ImgPreviewContainer({ attachmentsInputRef, attachmentFormRef }) {

    const newAttachmentsInput = useRef()
    const [attachments, setAttachments] = useState([])
    const [previews, setPreviews] = useState([])

    function onClickAddNewAttachment() {
        newAttachmentsInput.current.click()
    }

    function addNewAttachment(event) {
        let files = Array.prototype.map.call(newAttachmentsInput.current.files, x => x)
        setAttachments([ ...attachments, ...files])

        for (let i = 0; i < files.length; i++) {
            (function (i) {
                let file = files[i]
                const reader = new FileReader()
                console.log('file', file)
                reader.addEventListener('load', () => {
                    setPreviews([...previews, reader.result])
                }, false)
                reader.readAsDataURL(file)
            })(i)
        }
    }

    function removePreview(itemIdx) {
        let clonedPreviews = [...previews]
        let clonedAttachments = [...attachments]
        clonedPreviews.splice(itemIdx, 1)
        clonedAttachments.splice(itemIdx, 1)
        setPreviews(clonedPreviews)
        setAttachments(clonedAttachments)
        if (clonedPreviews.length === 0) {
            attachmentsInputRef.current.value === ''
            attachmentFormRef.current.reset()
        }
    }

    function onChangeAttachments(event) {
        if (!attachmentsInputRef || !attachmentsInputRef.current || !attachmentsInputRef.current.files || attachmentsInputRef.current.files.length === 0) {
            setAttachments([])
            setPreviews([])
            return
        }

        let files = Array.prototype.map.call(attachmentsInputRef.current.files, x => x)
        setAttachments(files)
        setPreviews(new Array(files.length))

        for (let i = 0; i < files.length; i++) {
            (function (i) {
                let file = files[i]
                if (isImage(file.type)) {
                    const reader = new FileReader()
                    reader.addEventListener('load', () => {
                        setPreviews(prev => {
                            prev[i] = reader.result
                            return [ ...prev ]
                        })
                    }, false)
                    reader.readAsDataURL(file)
                } else {
                    setPreviews(prev => {
                        prev[i] = 'file'
                        return [ ...prev ]
                    })
                }

            })(i)
        }
    }

    return (
        <ImgPreviewContainer show={previews.length > 0}>
            {previews.map((dataURL, idx) => !!dataURL ? (
                <SImgPreviewWrapper key={idx}>
                    <button onClick={() => removePreview(idx)}>
                        <CloseIcon />
                    </button>
                <div style={{ width: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <ImgPreview onError={(event) => { event.target.src = fileIcon }} src={dataURL} key={dataURL} />
                </div>

                <div style={{ width: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <span>{attachments[idx] ? attachments[idx].name : 'File'}</span>
                </div>
                </SImgPreviewWrapper>
            ) : null)}
            <SAddNewAttachment>
                <img src={uploadIcon} alt="Upload" />
                <Button
                    primary
                    style={{
                        width: '108px',
                        marginTop: 12,
                        fontSize: 10,
                        padding: '3px 6px',
                        paddingTop: '4px',
                        lineHeight: '1.25',
                    }}
                    onClick={onClickAddNewAttachment}
                >Add File(s)</Button>
            </SAddNewAttachment>
            <AdditionHiddenInput type="file" multiple ref={newAttachmentsInput} onChange={addNewAttachment} />
            <HiddenInput ref={attachmentFormRef}>
                <input type="file" multiple ref={attachmentsInputRef} onChange={onChangeAttachments} />
            </HiddenInput>
        </ImgPreviewContainer>
    )
}

export default ImgPreviewContainer
